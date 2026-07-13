package main

// cmd_tokens_coverage_test.go — cycle-779 TDD contract (RED) for the
// token-telemetry-input-cache-fidelity task's AC3 report half: `evolve tokens
// report` gains a coverage line — phases WITH token data / phases RUN — so a
// telemetry gap (the 2026-07-13 all-zeros baseline) is visible in the report
// itself instead of masquerading as "this phase is free".
//
// The contract is deliberately loose about layout: the rendered report must
// carry a line starting with "Coverage:" containing the `<with-data>/<run>`
// ratio. Builder owns the exact wording and any TokensReport JSON field.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// TestTokensReport_CoverageLinePresent: one phase with real token data and one
// that ran with zero tokens → the rendered report must state coverage 1/2.
func TestTokensReport_CoverageLinePresent(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "1", []phasetiming.Entry{
		{Phase: "build", DurationMS: 1000, Verdict: "PASS",
			Tokens: cyclestate.TokenUsage{Input: 2000, Output: 500, CacheRead: 30000, CacheWrite: 400}},
		{Phase: "scout", DurationMS: 1000, Verdict: "PASS"},
	})

	report := buildTokensReport(filepath.Join(root, ".evolve", "runs"), []int{1})
	var buf bytes.Buffer
	renderTokensReport(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "Coverage:") {
		t.Fatalf("rendered report has no Coverage: line (telemetry gaps stay invisible)\n%s", out)
	}
	if !strings.Contains(out, "1/2") {
		t.Errorf("Coverage line does not carry the 1/2 phases-with-data ratio\n%s", out)
	}
}

// TestTokensReport_CoverageCountsOnlyPhasesWithData (negative): a window where
// EVERY phase ran but NONE recorded tokens (the 2026-07-13 baseline shape)
// must report coverage 0/2 — zero-token phases are uncovered, never counted
// as covered-and-free.
func TestTokensReport_CoverageCountsOnlyPhasesWithData(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "1", []phasetiming.Entry{
		{Phase: "build", DurationMS: 1000, Verdict: "PASS"},
		{Phase: "audit", DurationMS: 1000, Verdict: "PASS"},
	})

	report := buildTokensReport(filepath.Join(root, ".evolve", "runs"), []int{1})
	var buf bytes.Buffer
	renderTokensReport(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "Coverage:") {
		t.Fatalf("rendered report has no Coverage: line for an all-zero window\n%s", out)
	}
	if !strings.Contains(out, "0/2") {
		t.Errorf("all-zero window must report coverage 0/2, not claim coverage\n%s", out)
	}
}
