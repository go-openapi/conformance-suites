// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package goopenapi reads a document with github.com/go-openapi/go-yaml, the parser the consensus report scores, by
// running the goyaml-json program.
//
// goyaml-json lives in its own module, yaml/consensus/docker/goyaml-json, so that this module does not require
// go-openapi/go-yaml: the parser under test changes far more often than the references it is compared with, and a
// requirement here would need a version bump on every change. The consensus image builds the program from a pinned
// version, or from a local checkout.
package goopenapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNotInstalled says the goyaml-json program could not be found.
var ErrNotInstalled = errors.New("goopenapi: goyaml-json is not installed; set GOYAML_JSON or put goyaml-json on PATH")

// Answer is what goyaml-json writes for one stream.
type Answer struct {
	Verdict   string   `json:"verdict"`
	RefusedBy string   `json:"refusedBy,omitempty"`
	Docs      int      `json:"docs,omitempty"`
	JSON      []string `json:"json,omitempty"`
	Message   string   `json:"message,omitempty"`
}

// Command is the goyaml-json program this package runs: GOYAML_JSON when set, else goyaml-json on PATH.
func Command() string {
	if set := os.Getenv("GOYAML_JSON"); set != "" {
		return set
	}
	found, err := exec.LookPath("goyaml-json")
	if err != nil {
		return ""
	}

	return found
}

// Available reports whether goyaml-json can be run.
func Available() bool { return Command() != "" }

// Version is the go-openapi/go-yaml version the program was built from, as "go version -m" reports it, or empty.
func Version() string {
	command := Command()
	if command == "" {
		return ""
	}
	out, err := exec.CommandContext(context.Background(), "go", "version", "-m", command).Output()
	if err != nil {
		return versionFile(command)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "dep" && fields[1] == "github.com/go-openapi/go-yaml" {
			if len(fields) >= 5 && fields[3] == "=>" {
				return fields[2] + " => " + fields[4]
			}

			return fields[2]
		}
	}

	return versionFile(command)
}

// versionFile reads the version the image wrote beside the program, where no Go toolchain is installed.
func versionFile(command string) string {
	out, err := os.ReadFile(command + ".version")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// Read runs src through goyaml-json and returns its answer.
func Read(ctx context.Context, src []byte) (Answer, error) {
	command := Command()
	if command == "" {
		return Answer{}, ErrNotInstalled
	}

	cmd := exec.CommandContext(ctx, command)
	cmd.Stdin = bytes.NewReader(src)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	if err := cmd.Run(); err != nil {
		return Answer{}, fmt.Errorf("goopenapi: %w: %s", err, strings.TrimSpace(errs.String()))
	}

	var a Answer
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &a); err != nil {
		return Answer{}, fmt.Errorf("goopenapi: reading its answer: %w", err)
	}

	return a, nil
}
