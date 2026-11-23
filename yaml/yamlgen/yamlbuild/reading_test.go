// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"slices"
	"testing"

	"github.com/go-openapi/conformance-suites/suite"

	yamlembed "github.com/go-openapi/conformance-suites/yaml/corpus"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
)

func storedCases(t *testing.T) (suite.Header, []suite.Case) {
	t.Helper()

	header, cases, err := yamlembed.CompactSuite()
	if err != nil {
		t.Fatalf("reading the stored corpus: %v", err)
	}

	return header, cases
}

// TestAReadingTheCorpusDoesNotStateIsUnscored is the honest half.
//
// Nothing here states an answer under the JSON schema, so a consumer
// implementing one is left unscored on values rather than measured against the
// core schema's answer. Silence is the right result and the false return is
// what carries it.
func TestAReadingTheCorpusDoesNotStateIsUnscored(t *testing.T) {
	var stated int

	header, cases := storedCases(t)

	for _, c := range cases {
		if _, ok := header.MeaningFor(c, yamlgen.ReadingJSON); ok {
			stated++
		}
	}

	if stated != 0 {
		t.Errorf("%d cases claim an answer under the JSON schema, and none should", stated)
	}
}

// TestAgreementNeedsNoList checks the cheap path.
//
// Most documents mean the same thing under every reading, carry one Meaning and
// no list at all, and every consumer is entitled to that one answer. Storing
// three copies of it would triple the artifact to say nothing.
func TestAgreementNeedsNoList(t *testing.T) {
	var agreed, listed int

	header, cases := storedCases(t)

	for _, c := range cases {
		if c.Meaning == nil {
			continue
		}

		if len(c.Meanings) > 0 {
			listed++

			continue
		}

		agreed++

		if _, ok := header.MeaningFor(c, yamlgen.Reading11); !ok {
			t.Fatalf("%s agrees under every reading and YAML 1.1 got nothing", c.Name)
		}
	}

	if listed == 0 || agreed == 0 {
		t.Errorf("the corpus has %d cases that agree and %d that do not; both should be non-zero",
			agreed, listed)
	}

	t.Logf("%d cases agree under every reading, %d state one answer per reading", agreed, listed)
}

// TestAnUnweighedReadingGetsNothingEvenWhereTheOthersAgree pins the trap.
//
// The failsafe schema reads every scalar as a string, so it disagrees with the
// core schema about "k: 1" and about most of this corpus. Almost none of those
// cases carries a list of meanings, because no reading the corpus weighed
// disagrees about them -- and reading that as "every reading agrees" would hand
// a failsafe consumer the integer and mark it broken for saying "1".
//
// Header.Readings is what separates "they agree" from "nobody asked", and this
// is the failure it prevents.
func TestAnUnweighedReadingGetsNothingEvenWhereTheOthersAgree(t *testing.T) {
	header, cases := storedCases(t)

	const failsafe = "yaml-1.2-failsafe"

	if slices.Contains(header.Readings, failsafe) {
		t.Fatalf("this corpus now weighs %s, so the test needs a reading it does not", failsafe)
	}

	for _, c := range cases {
		if c.Meaning == nil {
			continue
		}

		if _, ok := header.MeaningFor(c, failsafe); ok {
			t.Fatalf("%s answered for a reading this corpus never weighed", c.Name)
		}
	}

	// The same case under a reading it does weigh still answers, or the check
	// above would pass for the wrong reason.
	var answered int

	for _, c := range cases {
		if _, ok := header.MeaningFor(c, yamlgen.Reading11); ok {
			answered++
		}
	}

	if answered == 0 {
		t.Error("no case answers under YAML 1.1, so nothing above was actually exercised")
	}
}
