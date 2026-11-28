// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/conformance-suites/suite"
	stancefiles "github.com/go-openapi/conformance-suites/yaml/consensus/internal/stances"
)

// stanceFile is one parser's stance file, as written in internal/stances/<id>.yaml.
type stanceFile struct {
	// Parser is the source id the file describes.
	Parser string `yaml:"parser"`
	// At is how far the parser takes a document: parse, compose or construct.
	At string `yaml:"at"`
	// Because describes how the runner reads the document with this parser.
	Because string `yaml:"because"`
	// Stands are the parser's positions on tags: refuses, accepts or either, alone or with a because.
	Stands map[string]standFile `yaml:"stands"`
	// Departures are the known differences from the corpus that no stand covers.
	Departures []departureFile `yaml:"departures"`
}

// standFile is one stand: a bare word, or a mapping with the word and why.
type standFile struct {
	Stand   string `yaml:"stand"`
	Because string `yaml:"because"`
}

// UnmarshalYAML reads "refuses" as well as {stand: refuses, because: ...}.
func (s *standFile) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.Stand = node.Value

		return nil
	}
	type plain standFile

	return node.Decode((*plain)(s))
}

type departureFile struct {
	ID string `yaml:"id"`
	// Rejects is "valid" for a departure that rejects documents the corpus accepts.
	Rejects string `yaml:"rejects"`
	// Accepts is "invalid" for a departure that accepts documents the corpus rejects.
	Accepts string `yaml:"accepts"`
	// Differs is "value" for a departure that builds another value than the corpus states.
	Differs string    `yaml:"differs"`
	Match   matchFile `yaml:"match"`
	// Kind is position, leniency or defect.
	Kind    string `yaml:"kind"`
	Because string `yaml:"because"`
}

type matchFile struct {
	Tags     []string `yaml:"tags"`
	Features []string `yaml:"features"`
	Message  string   `yaml:"message"`
	Value    string   `yaml:"value"`
	Src      string   `yaml:"src"`
}

// Departure kinds.
const (
	kindPosition = "position"
	kindLeniency = "leniency"
	kindDefect   = "defect"
)

// parserStance is a stance file made ready to score with.
type parserStance struct {
	// Declared says whether a file declared this stance; without one the parser has only its stage.
	Declared bool
	Because  string
	Table    stance.Table
	// StandBecause holds the because of each stand that gives one.
	StandBecause map[stance.Tag]string
	// Rejecting and Accepting are the departures that explain a rejection of a valid document, and an acceptance
	// of an invalid one.
	Rejecting, Accepting []*departure
	// Differing are the departures that explain a value other than the corpus states.
	Differing []*departure
}

// departure is one known departure, with its match compiled.
type departure struct {
	ID, Kind, Because string
	tags, features    []string
	message, src      *regexp.Regexp
	value             *regexp.Regexp
}

// Matches says whether the departure explains a source's answer on a case.
func (d *departure) Matches(c record, a answerView) bool {
	for _, t := range d.tags {
		if !slices.Contains(c.Tags, t) {
			return false
		}
	}
	for _, f := range d.features {
		if !slices.Contains(c.Features, f) {
			return false
		}
	}
	if d.message != nil && !d.message.MatchString(a.Message) {
		return false
	}
	if d.value != nil && !d.value.MatchString(a.Value) {
		return false
	}
	if d.src != nil && !d.src.Match(c.Src) {
		return false
	}

	return true
}

// answerView is what a departure reads of an answer.
type answerView struct{ Message, Value string }

// atOf is the stage each source reaches when it has no stance file.
var atOf = map[string]stance.Stage{
	"perlref": stance.Parse, "libfyaml": stance.Parse,
	"yamlstar": stance.Construct, "yaml.v3": stance.Construct, "go-yaml": stance.Construct,
}

// loadStances reads the stance file of each source from dir, or from the embedded files when dir is empty, and
// builds each source's table from the corpus's rules.
func loadStances(dir string, h head) ([]parserStance, error) {
	var files fs.FS = stancefiles.Files
	if dir != "" {
		files = os.DirFS(dir)
	}

	rules, vocabulary := rulesOf(h.Corpus.Vocabulary)
	out := make([]parserStance, len(h.Sources))
	for i, s := range h.Sources {
		ps := parserStance{Table: stance.Table{
			Name: s.ID, At: atOf[s.ID], Requires: rules, Speaks: vocabulary, Stands: map[stance.Tag]stance.Stand{},
		}}
		raw, err := fs.ReadFile(files, s.ID+".yaml")
		switch {
		case errors.Is(err, fs.ErrNotExist):
			out[i] = ps

			continue
		case err != nil:
			return nil, err
		}

		var f stanceFile
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(&f); err != nil {
			return nil, fmt.Errorf("stance file of %s: %w", s.ID, err)
		}
		if err := ps.fill(f, vocabulary); err != nil {
			return nil, fmt.Errorf("stance file of %s: %w", s.ID, err)
		}
		out[i] = ps
	}

	return out, nil
}

func (ps *parserStance) fill(f stanceFile, vocabulary stance.Vocabulary) error {
	ps.Declared, ps.Because = true, f.Because
	if f.At != "" {
		at, ok := map[string]stance.Stage{"parse": stance.Parse, "compose": stance.Compose, "construct": stance.Construct}[f.At]
		if !ok {
			return fmt.Errorf("at: %q is not parse, compose or construct", f.At)
		}
		ps.Table.At = at
	}

	ps.StandBecause = map[stance.Tag]string{}
	for tag, entry := range f.Stands {
		st, ok := map[string]stance.Stand{"refuses": stance.Refuses, "accepts": stance.Accepts, "either": stance.Either}[entry.Stand]
		if !ok {
			return fmt.Errorf("stands: %s: %q is not refuses, accepts or either", tag, entry.Stand)
		}
		if entry.Because != "" {
			ps.StandBecause[stance.Tag(tag)] = entry.Because
		}
		if _, known := vocabulary[stance.Tag(tag)]; !known {
			return fmt.Errorf("stands: %s is not a tag of this corpus", tag)
		}
		ps.Table.Stands[stance.Tag(tag)] = st
	}

	for _, d := range f.Departures {
		dep, err := compileDeparture(d)
		if err != nil {
			return fmt.Errorf("departures: %s: %w", d.ID, err)
		}
		switch {
		case d.Rejects == "valid" && d.Accepts == "" && d.Differs == "":
			ps.Rejecting = append(ps.Rejecting, dep)
		case d.Accepts == "invalid" && d.Rejects == "" && d.Differs == "":
			ps.Accepting = append(ps.Accepting, dep)
		case d.Differs == "value" && d.Rejects == "" && d.Accepts == "":
			ps.Differing = append(ps.Differing, dep)
		default:
			return fmt.Errorf("departures: %s: give one of rejects: valid, accepts: invalid or differs: value", d.ID)
		}
	}

	return nil
}

// compileDeparture checks a departure and compiles its match.
func compileDeparture(d departureFile) (*departure, error) {
	if d.ID == "" || d.Because == "" {
		return nil, errors.New("each departure needs an id and a because")
	}
	if !slices.Contains([]string{kindPosition, kindLeniency, kindDefect}, d.Kind) {
		return nil, fmt.Errorf("kind %q is not position, leniency or defect", d.Kind)
	}
	dep := &departure{ID: d.ID, Kind: d.Kind, Because: d.Because, tags: d.Match.Tags, features: d.Match.Features}
	for _, field := range []struct {
		name, expr string
		into       **regexp.Regexp
	}{{"message", d.Match.Message, &dep.message}, {"value", d.Match.Value, &dep.value}, {"src", d.Match.Src, &dep.src}} {
		if field.expr == "" {
			continue
		}
		re, err := regexp.Compile(field.expr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.name, err)
		}
		*field.into = re
	}
	if len(dep.tags)+len(dep.features) == 0 && dep.message == nil && dep.src == nil && dep.value == nil {
		return nil, errors.New("the match is empty")
	}

	return dep, nil
}

// rulesOf reads the settled rules and the stages back from the corpus's vocabulary.
func rulesOf(specs []suite.TagSpec) (stance.Rules, stance.Vocabulary) {
	var rules stance.Rules
	vocabulary := stance.Vocabulary{}
	for _, spec := range specs {
		tag := stance.Tag(spec.Tag)
		vocabulary[tag] = suite.StageOf(spec.Stage)
		switch spec.Settled {
		case "reject":
			rules = append(rules, stance.Rule{Tag: tag, Then: stance.Reject, Because: spec.Because})
		case "accept":
			rules = append(rules, stance.Rule{Tag: tag, Then: stance.Accept, Because: spec.Because})
		}
	}

	return rules, vocabulary
}
