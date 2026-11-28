// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
)

// fakeDriverEnv makes the test binary act as a driver for fakeSource, so the runner's handling of a driver that
// panics, hangs or dies is tested on a real process.
const fakeDriverEnv = "CONSENSUS_FAKE_DRIVER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeDriverEnv) != "" {
		if err := drive(fakeSource(), time.Minute, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeSource accepts every document, except "panic", "hang" and "exit", which do what they say.
func fakeSource() source {
	return source{id: "fake", answer: func(_ context.Context, src []byte) consensus.Answer {
		switch string(src) {
		case "panic":
			panic("boom")
		case "hang":
			select {}
		case "exit":
			fmt.Fprintln(os.Stderr, "fatal error: stack overflow")
			os.Exit(2)
		}

		return consensus.Answer{Verdict: consensus.Accept, Docs: 1}
	}}
}

func runFake(t *testing.T, procs int, docs ...string) ([]record, runStats, string) {
	t.Helper()
	watchdogGrace = 200 * time.Millisecond
	t.Cleanup(func() { watchdogGrace = 5 * time.Second })

	records := make([]record, len(docs))
	for i, d := range docs {
		records[i] = record{SHA: fmt.Sprintf("sha-%d", i), Src: []byte(d), Answers: make([]consensus.Answer, 1)}
	}
	c, err := openCache("", "fake", "test")
	if err != nil {
		t.Fatal(err)
	}
	s := fakeSource()
	s.procs = procs
	stats, reason := runSource(s, 0, records, c, runOptions{
		timeout: 100 * time.Millisecond,
		driver: func(source) ([]string, []string) {
			return []string{os.Args[0]}, append(os.Environ(), fakeDriverEnv+"=1")
		},
	})

	return records, stats, reason
}

func TestADriverSurvivesAPanicAHangAndACrash(t *testing.T) {
	records, stats, reason := runFake(t, 1, "a", "panic", "b", "hang", "c", "exit", "d")
	if reason != "" {
		t.Fatalf("dropped: %s", reason)
	}

	want := []struct {
		verdict consensus.Verdict
		message string
	}{
		{consensus.Accept, ""},
		{consensus.Error, "panic: boom"},
		{consensus.Accept, ""},
		{consensus.Error, "timed out"},
		{consensus.Accept, ""},
		{consensus.Error, "crashed: fatal error: stack overflow"},
		{consensus.Accept, ""},
	}
	for i, w := range want {
		a := records[i].Answers[0]
		if a.Source != "fake" || a.Verdict != w.verdict || !strings.Contains(a.Message, w.message) {
			t.Errorf("case %q: got %s %q from %q, want %s %q", records[i].Src, a.Verdict, a.Message, a.Source, w.verdict, w.message)
		}
	}
	if stats.Asked != 5 || stats.Timeouts != 1 || stats.Crashes != 1 || stats.Procs != 1 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestASourceIsDroppedAfterTenCrashes(t *testing.T) {
	docs := make([]string, 15)
	for i := range docs {
		docs[i] = "exit"
	}
	_, stats, reason := runFake(t, 1, docs...)
	if reason == "" || stats.Crashes != maxCrashes {
		t.Errorf("got reason %q after %d crashes, want a drop after %d", reason, stats.Crashes, maxCrashes)
	}
}

func TestSideBySideDriversAnswerEveryCase(t *testing.T) {
	docs := make([]string, 20)
	for i := range docs {
		docs[i] = fmt.Sprintf("doc %d", i)
	}
	records, stats, _ := runFake(t, 3, docs...)
	for _, r := range records {
		if r.Answers[0].Verdict != consensus.Accept {
			t.Errorf("%q: %+v", r.Src, r.Answers[0])
		}
	}
	if stats.Asked != len(docs) || stats.Procs != 3 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestSideBySideKeepsTheCICountOnCI(t *testing.T) {
	t.Setenv("CI", "true")
	if got := sideBySide(4); got != 4 {
		t.Errorf("on CI: got %d drivers, want 4", got)
	}

	t.Setenv("CI", "")
	if got, want := sideBySide(4), min(localDrivers, runtime.NumCPU()); got != want {
		t.Errorf("locally: got %d drivers, want %d", got, want)
	}
}
