//go:build acs

// Package cycle1070 (amplification set) adds adversarial coverage the TDD
// contract's five predicates (predicates_test.go) do not enumerate, per the
// Test Amplifier's black-box mandate: derive cases from the spec in
// test-report.md ("## Task: tdd-topn-scope-gate" contract, the fail-open
// truth table, and the "Handoff to Builder" fixture shape), never from the
// Builder's diff. doNotModifyTests:true is honoured — nothing here edits an
// existing test; it only adds new ones reusing the existing writeTriage /
// writeTestReport / reviewTDD fixture helpers.
//
// Every new test drives the same exported seam as 001-005:
// topngate.NewReviewer(stage).Review(ctx, core.ReviewInput{...}) against a
// t.TempDir()-backed workspace. No test mutates the live repo tree.
package cycle1070

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// writeRawTestReport writes an arbitrary raw string as test-report.md,
// letting amplification tests construct malformed/adversarial shapes that the
// well-formed writeTestReport helper (predicates_test.go) cannot produce.
func writeRawTestReport(t *testing.T, ws, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write raw test-report.md: %v", err)
	}
}

// TestC1070_006_EmptyTopNNoAuthoredFilesApproves is the compliant-no-op edge
// AC1 does not itself cover: triage committed nothing AND TDD authored
// nothing. The spec's truth table lists this as an explicit pass row ("empty
// ## top_n + authored nothing | pass (compliant no-op)"), distinct from 001
// (empty top_n + non-empty testFiles, which must BLOCK).
func TestC1070_006_EmptyTopNNoAuthoredFilesApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)                  // empty ## top_n
	writeTestReport(t, ws, "some-slug") // zero testFiles — the compliant response

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("empty top_n + zero authored files is the compliant no-op and must be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_007_MissingTestReportFailsOpen covers the other half of the
// fail-open pair that 003 only tests one side of (missing triage-report.md).
// A missing test-report.md leaves nothing to bind a claimed slug from, so the
// gate must fail open rather than block.
func TestC1070_007_MissingTestReportFailsOpen(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-a", "committed-b")
	// Deliberately no test-report.md written.

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("missing test-report.md (nothing to bind a claimed slug from) must fail OPEN; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_008_UnparseableTaskHeaderFailsOpen locks the spec's explicit
// "authored files but no parseable ## Task: header -> fail open" row: a
// malformed report with a valid Handoff JSON fence but no "## Task:" header
// at all must not be treated as a certain violation.
func TestC1070_008_UnparseableTaskHeaderFailsOpen(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-a", "committed-b")
	raw := "# TDD Report — Cycle 1070\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": [\"go/acs/cycle1070/x_test.go\"]\n}\n```\n"
	writeRawTestReport(t, ws, raw)

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("authored files but no parseable ## Task: header must fail OPEN (nothing to bind against); got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_009_NonJSONFenceIgnoredNotFalsePositive replicates the real
// production test-report.md shape (this very cycle's report has a RED Run
// Output fence containing the literal path string
// "go/acs/cycle660/predicates_test.go" ahead of the JSON handoff fence). Only
// the "## Handoff to Builder" JSON fence is authoritative per spec; a parser
// that also scanned prose fences for path-shaped strings would false-block
// here even though the JSON declares zero authored files.
func TestC1070_009_NonJSONFenceIgnoredNotFalsePositive(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "some-other-committed-slug") // claimed slug below is out-of-lane
	raw := "# TDD Report — Cycle 1070\n\n## Task: unrelated-slug\n\n" +
		"## RED Run Output\n```\n" +
		"$ go test -tags acs -count=1 -v ./acs/cycle1070\n" +
		"    predicates_test.go:114: rejected go/acs/cycle660/predicates_test.go\n" +
		"FAIL\n```\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": []\n}\n```\n"
	writeRawTestReport(t, ws, raw)

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("the authoritative handoff JSON declares zero authored files; a non-JSON prose fence mentioning a file path must NOT cause a false block; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_010_MalformedJSONFenceDoesNotPanic is a negative/robustness case:
// a truncated, invalid JSON handoff fence must not crash the reviewer. The
// package's stated design ("fails open on every ambiguity") makes unparseable
// handoff JSON an ambiguity, so Approve=true is the expected safe default —
// but the hard requirement is no panic regardless.
func TestC1070_010_MalformedJSONFenceDoesNotPanic(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws) // empty top_n
	raw := "# TDD Report — Cycle 1070\n\n## Task: some-slug\n\n" +
		"## Handoff to Builder\n```json\n{\n  \"testFiles\": [\"unterminated.go\"\n```\n"
	writeRawTestReport(t, ws, raw)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("malformed handoff JSON must not panic the reviewer; recovered: %v", r)
		}
	}()
	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("unparseable handoff JSON is an ambiguity and this package fails open on every ambiguity; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_011_BlankStringTestFileEntryStillCountsAsAuthored distinguishes
// "array length zero" (compliant no-op, see 006) from "array length one
// containing a blank string" — per spec the signal is testFiles[] non-empty,
// an array-shape check, not a check on the contents' truthiness.
func TestC1070_011_BlankStringTestFileEntryStillCountsAsAuthored(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws)                          // empty top_n
	writeTestReport(t, ws, "declined-slug", "") // testFiles: [""] — length 1, not 0

	res := reviewTDD(t, config.StageEnforce, ws)
	if res.Approve {
		t.Errorf("a non-empty testFiles[] array (even a single blank-string element) under an empty top_n is authored-files-present and must be REJECTED; got Approve=true")
	}
	if !res.Approve && res.Reason == "" {
		t.Errorf("a blocked review must carry a non-empty Reason (core.ReviewResult contract); got empty Reason")
	}
}

// TestC1070_012_OutOfLaneSlugWithNoAuthoredFilesApproves isolates the second
// operand of the AC2 conjunction: the spec's BLOCK row requires BOTH
// zero-overlap AND authored files. Drop "authored files" and there is nothing
// to reject, even though the claimed slug is still out-of-lane.
func TestC1070_012_OutOfLaneSlugWithNoAuthoredFilesApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "committed-slug-a", "committed-slug-b")
	writeTestReport(t, ws, "totally-other-slug") // out-of-lane claim, zero authored files

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("an out-of-lane claimed slug with zero authored testFiles[] has nothing to reject; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_013_LargeScaleTopNAndTestFilesApproves is the limit/large-scale
// case: a 500-entry committed top_n and 200 authored files must parse and
// resolve correctly, not just the 1-2 entry fixtures 001-005 use.
func TestC1070_013_LargeScaleTopNAndTestFilesApproves(t *testing.T) {
	ws := t.TempDir()
	slugs := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		slugs = append(slugs, fmt.Sprintf("slug-%04d", i))
	}
	writeTriage(t, ws, slugs...)

	files := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		files = append(files, fmt.Sprintf("go/acs/cycle1070/gen_%04d_test.go", i))
	}
	writeTestReport(t, ws, "slug-0499", files...) // claimed slug near the end of a large committed set

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("in-lane claim against a large (500-entry) committed top_n with 200 authored files must still be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}

// TestC1070_014_DuplicateSlugInTopNStillApproves is a parser-robustness edge:
// a duplicated committed-slug bullet (e.g. from a triage authoring slip) must
// not break membership resolution for an otherwise-valid in-lane claim.
func TestC1070_014_DuplicateSlugInTopNStillApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriage(t, ws, "tdd-topn-scope-gate", "tdd-topn-scope-gate", "other-slug")
	writeTestReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1070/predicates_test.go")

	res := reviewTDD(t, config.StageEnforce, ws)
	if !res.Approve {
		t.Errorf("a duplicate committed-slug bullet must not break membership parsing; in-lane claim must be APPROVED; got Approve=false reason=%q", res.Reason)
	}
}
