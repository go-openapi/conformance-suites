// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/go-openapi/conformance-suites/suite"
	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

// head is the first line of a results file: the corpus and the sources it was run through.
type head struct {
	Corpus  corpusInfo   `json:"corpus"`
	Sources []sourceInfo `json:"sources"`
	Started time.Time    `json:"started"`
	Elapsed string       `json:"elapsed,omitempty"`
}

type corpusInfo struct {
	Grammar    string          `json:"grammar"`
	Digest     string          `json:"digest"`
	Generator  string          `json:"generator"`
	Tier       string          `json:"tier"`
	Cases      int             `json:"cases"`
	Vocabulary []suite.TagSpec `json:"vocabulary"`
	Readings   []string        `json:"readings,omitempty"`
	// Ref names the conformance-suites commit or tag the corpus file comes from, when the run is given one.
	Ref string `json:"ref,omitempty"`
}

type sourceInfo struct {
	ID        string `json:"id"`
	What      string `json:"what"`
	Version   string `json:"version"`
	UnderTest bool   `json:"underTest,omitempty"`
	Events    bool   `json:"events,omitempty"`
	Values    bool   `json:"values,omitempty"`
}

// record is one case of the corpus and every source's answer for it.
type record struct {
	Name       string             `json:"name"`
	SHA        string             `json:"sha"`
	Src        []byte             `json:"src"`
	WellFormed bool               `json:"wellFormed"`
	Opaque     bool               `json:"opaque,omitempty"`
	Tags       []string           `json:"tags,omitempty"`
	Features   []string           `json:"features,omitempty"`
	VerdictAt  string             `json:"verdictAt,omitempty"`
	Meaning    *suite.Meaning     `json:"meaning,omitempty"`
	Answers    []consensus.Answer `json:"answers"`
}

type runOptions struct {
	sources []source
	workers int
	timeout time.Duration
	cache   string
	// limit stops after the first limit cases, for a quick look; zero reads them all.
	limit int
	// corpusRef is the conformance-suites commit or tag the corpus file comes from, recorded in the results.
	corpusRef string
}

// runCorpus runs every case of the corpus at path through every source and writes the results to w.
func runCorpus(path string, opts runOptions, w io.Writer, progress io.Writer) error {
	h, cases, err := readCorpus(path, opts.limit)
	if err != nil {
		return err
	}

	infos := make([]sourceInfo, len(opts.sources))
	caches := make([]*cache, len(opts.sources))
	for i, s := range opts.sources {
		infos[i] = sourceInfo{
			ID: s.id, What: s.what, Version: s.version(), UnderTest: s.underTest, Events: s.events, Values: s.values,
		}
		c, err := openCache(opts.cache, s.id, infos[i].Version)
		if err != nil {
			return err
		}
		defer c.close()
		caches[i] = c
	}

	started := time.Now()
	records := make([]record, len(cases))
	for i, c := range cases {
		sum := sha256.Sum256(c.Src)
		records[i] = record{
			Name: c.Name, SHA: hex.EncodeToString(sum[:]), Src: c.Src, WellFormed: c.WellFormed, Opaque: c.Opaque,
			Tags: c.Tags, Features: c.Features, VerdictAt: c.VerdictAt, Meaning: c.Meaning,
			Answers: make([]consensus.Answer, len(opts.sources)),
		}
	}

	type job struct{ c, s int }
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for range max(opts.workers, 1) {
		wg.Go(func() {
			for j := range jobs {
				rec := &records[j.c]
				if a, ok := caches[j.s].get(rec.SHA); ok {
					rec.Answers[j.s] = a
				} else {
					a := timed(opts.sources[j.s], rec.Src, opts.timeout)
					rec.Answers[j.s] = a
					caches[j.s].put(rec.SHA, a)
				}

				mu.Lock()
				done++
				if progress != nil && done%2000 == 0 {
					fmt.Fprintf(progress, "consensus: %d of %d answers\n", done, len(records)*len(opts.sources))
				}
				mu.Unlock()
			}
		})
	}
	for c := range records {
		for s := range opts.sources {
			jobs <- job{c, s}
		}
	}
	close(jobs)
	wg.Wait()

	out := bufio.NewWriter(w)
	enc := json.NewEncoder(out)
	if err := enc.Encode(head{
		Corpus: corpusInfo{
			Grammar: h.Grammar, Digest: h.Digest, Generator: h.Generator, Tier: h.Tier, Cases: h.Cases,
			Vocabulary: h.Vocabulary, Readings: h.Readings, Ref: opts.corpusRef,
		},
		Sources: infos, Started: started.UTC(), Elapsed: time.Since(started).Round(time.Second).String(),
	}); err != nil {
		return err
	}
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}

	return out.Flush()
}

// readCorpus reads a corpus file, checks its format, and returns its header and its first limit cases, or all of
// them when limit is zero.
func readCorpus(path string, limit int) (suite.Header, []suite.Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return suite.Header{}, nil, err
	}
	defer f.Close()

	r, err := suite.NewReader(f)
	if err != nil {
		return suite.Header{}, nil, err
	}
	h := r.Header()
	if h.Format != suite.Format {
		return suite.Header{}, nil, fmt.Errorf("%s: format %d, this runner reads %d", path, h.Format, suite.Format)
	}

	var cases []suite.Case
	for {
		c, ok, err := r.Next()
		if err != nil {
			return suite.Header{}, nil, err
		}
		if !ok || (limit > 0 && len(cases) == limit) {
			return h, cases, nil
		}
		cases = append(cases, c)
	}
}

// cache keeps one source's answers by the SHA-256 of the document, in a JSON Lines file named after the source and
// its version, so a rerun after a corpus change only runs the documents it has not seen.
type cache struct {
	mu      sync.Mutex
	answers map[string]consensus.Answer
	file    *os.File
	enc     *json.Encoder
}

type cached struct {
	SHA    string           `json:"sha"`
	Answer consensus.Answer `json:"answer"`
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// cacheFormat names the shape of a cached answer. Raise it whenever an adapter or CanonicalJSON writes an answer
// differently, so that a cache written the old way is left behind instead of compared with new answers.
const cacheFormat = "3"

func openCache(dir, id, version string) (*cache, error) {
	c := &cache{answers: map[string]consensus.Answer{}}
	if dir == "" {
		return c, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	name := filepath.Join(dir, unsafeName.ReplaceAllString("v"+cacheFormat+"-"+id+"@"+version, "_")+".jsonl")
	if f, err := os.Open(name); err == nil {
		dec := json.NewDecoder(f)
		for {
			var e cached
			if err := dec.Decode(&e); err != nil {
				if !errors.Is(err, io.EOF) {
					fmt.Fprintf(os.Stderr, "consensus: %s: stopped reading the cache: %v\n", name, err)
				}

				break
			}
			c.answers[e.SHA] = e.Answer
		}
		f.Close()
	}

	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	c.file, c.enc = f, json.NewEncoder(f)

	return c, nil
}

func (c *cache) get(sha string) (consensus.Answer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.answers[sha]

	return a, ok
}

// put records an answer, except an error: a crash or a timeout says nothing about the document, and the next run
// should ask again.
func (c *cache) put(sha string, a consensus.Answer) {
	if a.Verdict == consensus.Error {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.answers[sha] = a
	if c.enc != nil {
		if err := c.enc.Encode(cached{SHA: sha, Answer: a}); err != nil {
			fmt.Fprintf(os.Stderr, "consensus: writing the cache: %v\n", err)
		}
	}
}

func (c *cache) close() {
	if c.file != nil {
		_ = c.file.Close()
	}
}
