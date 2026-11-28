// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
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
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
)

// head is the first line of a results file: the corpus and the sources it was run through.
type head struct {
	Corpus  corpusInfo   `json:"corpus"`
	Sources []sourceInfo `json:"sources"`
	// Dropped lists the sources whose drivers crashed too often: the results hold none of their answers.
	Dropped []dropped `json:"dropped,omitempty"`
	Started time.Time `json:"started"`
	Elapsed string    `json:"elapsed,omitempty"`
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
	ID        string   `json:"id"`
	What      string   `json:"what"`
	Version   string   `json:"version"`
	Events    bool     `json:"events,omitempty"`
	Values    bool     `json:"values,omitempty"`
	RoundTrip bool     `json:"roundTrip,omitempty"`
	Run       runStats `json:"run"`
}

// runStats records what running one source cost: the cases its drivers answered, the cases the cache answered,
// and the time spent.
type runStats struct {
	// Asked is how many cases the drivers answered; Cached is how many the cache answered instead.
	Asked  int `json:"asked"`
	Cached int `json:"cached"`
	// Seconds adds up the time the drivers took per case, from reading a case to writing its answer. It leaves out
	// starting the drivers and sending them the cases.
	Seconds float64 `json:"seconds"`
	// Wall is the time from the first case sent to the last answer read, across every driver of the source.
	Wall string `json:"wall"`
	// Procs is how many drivers ran side by side.
	Procs    int `json:"procs"`
	Crashes  int `json:"crashes,omitempty"`
	Timeouts int `json:"timeouts,omitempty"`
}

// Summary returns the run's cost as the report shows it, such as "24.6 s for 28279 documents, 1m2s wall on 4
// drivers". The time adds up each document's, so it does not depend on how many drivers ran. A source the cache
// answered in full shows "every document from the cache".
func (r runStats) Summary() string {
	if r.Asked == 0 {
		if r.Cached == 0 {
			return "not recorded"
		}

		return "every document from the cache"
	}

	drivers := "1 driver"
	if r.Procs > 1 {
		drivers = fmt.Sprintf("%d drivers", r.Procs)
	}
	line := fmt.Sprintf("%.1f s for %d documents, %s wall on %s", r.Seconds, r.Asked, r.Wall, drivers)
	if r.Cached > 0 {
		line += fmt.Sprintf("; %d more from the cache", r.Cached)
	}
	if r.Crashes > 0 {
		line += fmt.Sprintf("; %d crashes", r.Crashes)
	}
	if r.Timeouts > 0 {
		line += fmt.Sprintf("; %d timeouts", r.Timeouts)
	}

	return line
}

// dropped is a source left out of the results, and why.
type dropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
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
	timeout time.Duration
	cache   string
	// limit stops after the first limit cases, for a quick look; zero reads them all.
	limit int
	// corpusRef is the conformance-suites commit or tag the corpus file comes from, recorded in the results.
	corpusRef string
	// events keeps each source's event stream in its answers; the report reads none.
	events bool
	// driver returns the command line and the environment of a driver for source s. Nil runs this executable's
	// drive subcommand; a nil environment inherits the runner's.
	driver func(s source) (argv, env []string)
}

// maxCrashes is how many times a source's drivers may crash before the run drops the source.
const maxCrashes = 10

// watchdogGrace is added to the timeout a driver enforces itself, before the runner kills it. A source run inside
// the driver's process, such as yaml.v3, cannot be stopped from within.
var watchdogGrace = 5 * time.Second

// runCorpus runs every case of the corpus at path through every source and writes the results to w.
//
// The sources run one after another, each through its own driver processes (see [source.procs]), so a source that
// panics or hangs costs its own answers and nothing else.
func runCorpus(path string, opts runOptions, w io.Writer, progress io.Writer) error {
	h, cases, err := readCorpus(path, opts.limit)
	if err != nil {
		return err
	}
	if opts.driver == nil {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		opts.driver = func(s source) ([]string, []string) {
			argv := []string{self, "drive", "-timeout", opts.timeout.String()}
			if opts.events {
				argv = append(argv, "-events")
			}

			return append(argv, s.id), nil
		}
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

	infos := make([]sourceInfo, 0, len(opts.sources))
	var gone []dropped
	keep := make([]bool, len(opts.sources))
	for i, s := range opts.sources {
		info := sourceInfo{
			ID: s.id, What: s.what, Version: s.version(), Events: s.events, Values: s.values, RoundTrip: s.roundTrip,
		}
		c, err := openCache(opts.cache, s.id, info.Version)
		if err != nil {
			return err
		}
		stats, reason := runSource(s, i, records, c, opts)
		c.close()
		info.Run = stats
		if progress != nil {
			fmt.Fprintf(progress, "consensus: %s: %d asked, %d cached, %.1fs in the drivers, %s wall\n",
				s.id, stats.Asked, stats.Cached, stats.Seconds, stats.Wall)
		}
		if reason != "" {
			gone = append(gone, dropped{ID: s.id, Reason: reason})
			if progress != nil {
				fmt.Fprintf(progress, "consensus: %s dropped: %s\n", s.id, reason)
			}

			continue
		}
		keep[i] = true
		infos = append(infos, info)
	}
	for i := range records {
		kept := records[i].Answers[:0]
		for j, a := range records[i].Answers {
			if keep[j] {
				kept = append(kept, a)
			}
		}
		records[i].Answers = kept
	}

	out := bufio.NewWriter(w)
	enc := json.NewEncoder(out)
	if err := enc.Encode(head{
		Corpus: corpusInfo{
			Grammar: h.Grammar, Digest: h.Digest, Generator: h.Generator, Tier: h.Tier, Cases: h.Cases,
			Vocabulary: h.Vocabulary, Readings: h.Readings, Ref: opts.corpusRef,
		},
		Sources: infos, Dropped: gone, Started: started.UTC(), Elapsed: time.Since(started).Round(time.Second).String(),
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

// runSource fills the answers of source s, at index si of each record, from the cache or from its drivers. It
// returns what the run cost, and a reason when the source crashed too often to keep.
//
// Each of the source's drivers takes every n-th case, n being [source.procs]. A driver that crashes or times out
// answers an error for that case, and starts again at its next case.
func runSource(s source, si int, records []record, c *cache, opts runOptions) (runStats, string) {
	var pending []int
	stats := runStats{Procs: max(s.procs, 1)}
	for i := range records {
		rec := &records[i]
		// An answer cached before the source checked round trips lacks one, and is asked again.
		if a, ok := c.get(rec.SHA); ok && (!s.roundTrip || a.Verdict != consensus.Accept || a.RoundTrip != nil) {
			rec.Answers[si] = a
			stats.Cached++

			continue
		}
		pending = append(pending, i)
	}

	argv, env := opts.driver(s)
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		crashes int
	)
	start := time.Now()
	for k := range stats.Procs {
		wg.Go(func() {
			p := newProcess(argv, env)
			defer p.close()
			for n := k; n < len(pending); n += stats.Procs {
				mu.Lock()
				stop := crashes >= maxCrashes
				mu.Unlock()
				if stop {
					return
				}

				rec := &records[pending[n]]
				r, err := p.ask(request{SHA: rec.SHA, Src: rec.Src}, opts.timeout+watchdogGrace)
				a := r.Answer
				mu.Lock()
				switch {
				case errors.Is(err, errTimedOut):
					a = failed(context.DeadlineExceeded)
					stats.Timeouts++
				case err != nil:
					a = consensus.Answer{Verdict: consensus.Error, Message: consensus.FirstLine(err.Error())}
					crashes++
					stats.Crashes++
				default:
					stats.Asked++
					stats.Seconds += r.Seconds
				}
				mu.Unlock()
				a.Source = s.id
				rec.Answers[si] = a
				c.put(rec.SHA, a)
			}
		})
	}
	wg.Wait()
	stats.Wall = time.Since(start).Round(time.Millisecond).String()

	if crashes >= maxCrashes {
		return stats, fmt.Sprintf("its drivers crashed %d times", crashes)
	}

	return stats, ""
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
const cacheFormat = "4"

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
