// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Command consensus runs a YAML corpus through several YAML implementations and reports where they read it
// differently.
//
// The implementations are the Perl reference parser, libfyaml, YAMLStar's own parser, go.yaml.in/yaml/v3 and
// github.com/go-openapi/go-yaml, the parser under test. They are not votes: the report shows the routes each takes
// through a document, groups the sources by the answer they give, and scores each against the corpus alone.
//
// Usage:
//
//	consensus check
//	consensus run [-sources perlref,libfyaml,...] [-j N] [-timeout 10s] [-cache DIR] corpus.jsonl.gz > results.jsonl
//	consensus report [-o DIR] results.jsonl
//
// check lists the sources and whether each can be run. run writes one line naming the corpus and the sources, then
// one line per case with every source's answer. report reads those results and writes a static site: a summary, a
// grid by tag, a grid by feature, a page per source, and a page per case some source reads otherwise.
//
// The external sources are found as the adapters find them: REFPARSER_HOME, LIBFYAML_HOME, FY_TOOL, YAMLSTAR and
// GOYAML_JSON, or the default locations and PATH. The consensus image sets all of them.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "check":
		err = check()
	case "run":
		err = runCommand(os.Args[2:])
	case "report":
		err = reportCommand(os.Args[2:])
	case "-h", "-help", "--help", "help":
		usage()

		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensus: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  consensus check
  consensus run [-sources perlref,libfyaml,yamlstar,yaml.v3,go-yaml] [-j N] [-timeout 10s] [-cache DIR] corpus.jsonl.gz > results.jsonl
  consensus report [-o DIR] results.jsonl
`)
}

func check() error {
	missing := 0
	for _, s := range allSources() {
		state, version := "available", s.version()
		if !s.available() {
			state, missing = "missing", missing+1
		}
		fmt.Fprintf(os.Stdout, "%-9s %-9s %s\n          %s\n", s.id, state, version, s.what)
	}
	if missing > 0 {
		return fmt.Errorf("%d sources are missing", missing)
	}

	return nil
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	names := fs.String("sources", "", "comma-separated source ids; empty runs every available source")
	workers := fs.Int("j", runtime.NumCPU(), "how many answers to compute at once")
	timeout := fs.Duration("timeout", 10*time.Second, "how long one source may take on one document")
	cacheDir := fs.String("cache", "", "directory keeping each source's answers between runs")
	limit := fs.Int("limit", 0, "stop after this many cases, for a quick look; 0 runs them all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("run takes one corpus file")
	}

	var chosen []source
	want := strings.Split(*names, ",")
	for _, s := range allSources() {
		if *names != "" && !slices.Contains(want, s.id) {
			continue
		}
		if !s.available() {
			if *names != "" {
				return fmt.Errorf("%s is not available; see consensus check", s.id)
			}
			fmt.Fprintf(os.Stderr, "consensus: %s is not available, left out\n", s.id)

			continue
		}
		chosen = append(chosen, s)
	}
	if len(chosen) == 0 {
		return errors.New("no source is available; see consensus check")
	}

	return runCorpus(fs.Arg(0), runOptions{sources: chosen, workers: *workers, timeout: *timeout, cache: *cacheDir, limit: *limit},
		os.Stdout, os.Stderr)
}

func reportCommand(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	dir := fs.String("o", "report", "directory to write the report into")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("report takes one results file")
	}
	if err := writeReport(fs.Arg(0), *dir); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "consensus: wrote %s/index.html\n", *dir)

	return nil
}
