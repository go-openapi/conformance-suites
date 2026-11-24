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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
//
// A program built from a local checkout carries a replace, and its version names the directory, not the code:
// "v0.0.0-... => /src/go-yaml" for every checkout the image is given. The runner keys its cache on the version, so
// two checkouts shared one cache, and a run over a fix read back the answers of the build before it. Version
// appends the first 12 hex digits of the program's SHA-256 to a local build's version. goyaml-json is built with
// -trimpath, so the digest moves with the compiled code and with nothing else.
func Version() string {
	command := Command()
	if command == "" {
		return ""
	}

	version := moduleVersion(command)
	if version == "" || !isLocalBuild(version) {
		return version
	}

	digest, err := fileDigest(command)
	if err != nil {
		return version
	}

	return version + " (build " + digest + ")"
}

// moduleVersion returns go-openapi/go-yaml's version from "go version -m", or from the image's version file where
// no Go toolchain is installed.
func moduleVersion(command string) string {
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

// isLocalBuild reports whether version names a build from a local checkout: a replace, or the image's
// "local checkout" line.
func isLocalBuild(version string) bool {
	return strings.Contains(version, " => ") || strings.Contains(version, "local checkout")
}

// fileDigest returns the first 12 hex digits of the SHA-256 of the file at path.
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil))[:12], nil
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
