// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package libfyaml reads a document with libfyaml, through its Python binding and fy-tool.
//
// The public package github.com/go-openapi/conformance-suites/yaml/consensus/libfyaml documents the source and
// re-exports what other modules use.
package libfyaml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotInstalled says libfyaml could not be found.
//
// A test that wants it skips on this rather than failing: the harness has to
// run on a machine that has never installed it, and a conformance suite that
// cannot be run without a C library is one nobody runs.
var ErrNotInstalled = errors.New("libfyaml: not installed, run hack/conformance/install-libfyaml.sh")

// Home is where the install script puts the bindings.
//
// LIBFYAML_HOME overrides it, and the script reads the same variable, so the
// two cannot drift apart.
func Home() string {
	if set := os.Getenv("LIBFYAML_HOME"); set != "" {
		return set
	}

	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		cache = filepath.Join(home, ".cache")
	}

	return filepath.Join(cache, "go-openapi", "libfyaml")
}

// Available reports whether libfyaml can be reached.
func Available() bool { return Version() != "" }

// Version is the installed version, empty when there is none.
func Version() string {
	out, err := run(context.Background(), `import libfyaml; print(getattr(libfyaml, "__version__", "unknown"))`, "")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// Mode is libfyaml's own "mode" argument: the YAML version it reads a document
// under. It selects the resolution table and, with it, whether "<<" is a merge
// key.
type Mode string

const (
	// YAML12 is libfyaml's default: the 1.2 core schema, where "<<" is an
	// ordinary key.
	YAML12 Mode = "1.2"
	// YAML11 reads the 1.1 types, which is the only mode that resolves a
	// merge. Use it for a question about what "<<" means and for nothing else.
	YAML11 Mode = "1.1"
)

// Load reads src under YAML 1.2 and returns what libfyaml makes of each
// document in it, rendered as JSON.
//
// A document libfyaml refuses returns an error naming what it said. Use
// [LoadMode] with [YAML11] when the question is what a merge means.
func Load(src []byte) ([]string, error) { return LoadMode(src, YAML12) }

// LoadMode is [Load] under a named YAML version.
func LoadMode(src []byte, mode Mode) ([]string, error) {
	return LoadContext(context.Background(), src, mode)
}

// LoadContext is [LoadMode], stopped when ctx is done.
func LoadContext(ctx context.Context, src []byte, mode Mode) ([]string, error) {
	const script = `
import json, sys
import libfyaml

mode = sys.argv[1] if len(sys.argv) > 1 else "1.2"
src = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
out = [libfyaml.json_dumps(doc) for doc in libfyaml.loads_all(src, mode=mode)]
print(json.dumps(out))
`

	out, err := run(ctx, script, string(src), string(mode))
	if err != nil {
		return nil, err
	}

	var docs []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &docs); err != nil {
		return nil, fmt.Errorf("libfyaml: reading back its answer: %w", err)
	}

	return docs, nil
}

func run(ctx context.Context, script, stdin string, args ...string) (string, error) {
	home := Home()
	if home == "" {
		return "", ErrNotInstalled
	}

	if _, err := os.Stat(filepath.Join(home, "libfyaml")); err != nil {
		return "", ErrNotInstalled
	}

	argv := append([]string{"-c", script}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+home)
	cmd.Stdin = strings.NewReader(stdin)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("libfyaml: %w: %s", err, strings.TrimSpace(errs.String()))
	}

	return out.String(), nil
}

// ErrNoTool says fy-tool, libfyaml's command-line tool, could not be found.
var ErrNoTool = errors.New("libfyaml: fy-tool is not installed; set FY_TOOL or put fy-tool on PATH")

// Tool is the fy-tool this package runs for events: FY_TOOL when set, else fy-tool on PATH.
//
// The Python binding has no event API, so the events come from libfyaml's own command-line tool. Build it from the
// same release as the binding, so that both answers come from one version.
func Tool() string {
	if set := os.Getenv("FY_TOOL"); set != "" {
		return set
	}
	found, err := exec.LookPath("fy-tool")
	if err != nil {
		return ""
	}

	return found
}

// ToolVersion is what "fy-tool --version" prints, or empty when there is no tool.
func ToolVersion() string {
	tool := Tool()
	if tool == "" {
		return ""
	}
	out, err := exec.CommandContext(context.Background(), tool, "--version").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// Events runs src through "fy-tool --testsuite" and returns the event stream in test.event form.
//
// ok is false when libfyaml refused the document, and msg is what it said. err is a failure to run at all.
func Events(ctx context.Context, src []byte) (events []string, ok bool, msg string, err error) {
	tool := Tool()
	if tool == "" {
		return nil, false, "", ErrNoTool
	}

	cmd := exec.CommandContext(ctx, tool, "--testsuite")
	cmd.Stdin = bytes.NewReader(src)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	if runErr := cmd.Run(); runErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](runErr); !exited {
			return nil, false, "", fmt.Errorf("libfyaml: %w", runErr)
		}

		return nil, false, strings.TrimSpace(errs.String()), nil
	}

	for line := range strings.SplitSeq(strings.TrimRight(out.String(), "\n"), "\n") {
		if line != "" {
			events = append(events, line)
		}
	}

	return events, true, "", nil
}
