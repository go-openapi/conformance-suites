// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package jsonlines runs a long-lived child process that answers one JSON line with one JSON line.
//
// The consensus runner uses it to drive each source's driver, and the drivers use it to keep one Perl or Python
// interpreter alive for a whole corpus, instead of starting one per document.
package jsonlines

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"sync"
)

// ErrCrashed says the child ended, or wrote something other than one JSON line, before it answered.
var ErrCrashed = errors.New("crashed")

// MaxLine bounds one line read from the child.
const MaxLine = 64 << 20

// Child is a process started on the first [Child.Ask], and again on the next one after it crashed or was stopped.
//
// A Child serves one caller at a time.
type Child struct {
	// Argv is the command line. Env is its environment; nil inherits the caller's.
	Argv []string
	Env  []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan []byte
	done   chan struct{}
	stderr *tail
}

// Ask writes in as one JSON line and decodes the child's next line into out.
//
// When ctx ends first, Ask stops the child and returns ctx's error. When the child ends or answers with something
// other than JSON, Ask stops it and returns [ErrCrashed], wrapped with the line of its stderr that says why.
func (c *Child) Ask(ctx context.Context, in, out any) error {
	if c.cmd == nil {
		if err := c.start(); err != nil { //nolint:contextcheck // the child outlives the request; see start
			return fmt.Errorf("%w: cannot start %s: %w", ErrCrashed, c.Argv[0], err)
		}
	}

	line, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("%w: %s", ErrCrashed, c.Stop())
	}

	select {
	case got, ok := <-c.lines:
		if !ok {
			return fmt.Errorf("%w: %s", ErrCrashed, c.Stop())
		}
		if err := json.Unmarshal(got, out); err != nil {
			return fmt.Errorf("%w: answered %.80q: %s", ErrCrashed, got, c.Stop())
		}

		return nil
	case <-ctx.Done():
		c.Stop()

		return ctx.Err()
	}
}

// Stop kills the child, if it runs, and returns why it stopped, read from its stderr.
func (c *Child) Stop() string {
	if c.cmd == nil {
		return ""
	}
	close(c.done)
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	_ = c.cmd.Wait()
	c.cmd = nil

	return c.stderr.reason()
}

// Close ends the child after its last request: it sees the end of its input and exits.
func (c *Child) Close() {
	if c.cmd == nil {
		return
	}
	_ = c.stdin.Close()
	_ = c.cmd.Wait()
	close(c.done)
	c.cmd = nil
}

func (c *Child) start() error {
	// A background context, not the caller's: the child outlives one request, and Stop kills it.
	cmd := exec.CommandContext(context.Background(), c.Argv[0], c.Argv[1:]...) //nolint:gosec // a command this module builds
	cmd.Env = c.Env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	c.stderr = &tail{}
	cmd.Stderr = c.stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	lines, done := make(chan []byte), make(chan struct{})
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 1<<16), MaxLine)
		for sc.Scan() {
			select {
			case lines <- append([]byte(nil), sc.Bytes()...):
			case <-done:
				return
			}
		}
	}()

	c.cmd, c.stdin, c.lines, c.done = cmd, stdin, lines, done

	return nil
}

// tail keeps the end of what a child writes on stderr, enough to say why it crashed.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailSize = 4096

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, b...)
	if len(t.buf) > tailSize {
		t.buf = t.buf[len(t.buf)-tailSize:]
	}

	return len(b), nil
}

// reason returns the last line naming a Go fatal error or panic, which a stack trace follows, or else the last
// non-empty line, or "no message".
func (t *tail) reason() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	lines := strings.Split(string(t.buf), "\n")
	for _, line := range slices.Backward(lines) {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "fatal error: ") || strings.HasPrefix(l, "panic: ") {
			return l
		}
	}
	for _, line := range slices.Backward(lines) {
		if l := strings.TrimSpace(line); l != "" {
			return l
		}
	}

	return "no message"
}
