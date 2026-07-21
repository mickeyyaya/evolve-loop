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

// --- cycle-1014: report-level telemetry-coverage tripwire regression lock ---
//
// tripwire-regression-lock (inbox telemetry-coverage-tripwire-nonclaude-success,
// weight 0.93). Production is already landed and unchanged — the engine records
// `"tripwire":true` in llm-calls.ndjson (recordTokenUsage) and cycle-1013 wired
// `evolve tokens report` to read and surface it (readCycleTripwires /
// renderTripwires). This cycle adds the single explicitly-AC-named consolidated
// positive+negative regression at the report layer so a future hostile edit to
// the render path (the exact cycle-1007 render-order defect) is caught by a test
// whose name states the AC1/AC2/AC3 contract. The ACS predicate
// (go/acs/cycle1014/predicates_test.go) requires these two names verbatim.
// Fixture helpers (writeTokensTimingFixture / writeTokensLLMCalls /
// lineContaining) are shared from cmd_tokens_test.go (same package main).

// TestTokensReport_TripwireFiresOnNonClaudeSuccess — AC1+AC2. A single non-claude
// launch that exited 0, ran past the 60s success threshold, and resolved to
// source=none surfaces a TRIPWIRE line in the plain-text report, and that line
// names the offending CLI, agent, AND cycle together (not just one). Cycle 6 is
// discovered via its phase-timing.json; duration 90000 / exit 0 carry no stray
// "6" digit, so the cycle-number assertion is real.
func TestTokensReport_TripwireFiresOnNonClaudeSuccess(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "6", []phasetiming.Entry{
		{Phase: "audit", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 100, Output: 10}},
	})
	writeTokensLLMCalls(t, root, "6", []map[string]any{
		{"agent": "auditor", "phase": "audit", "cli": "agy", "source": "none",
			"duration_ms": 90000, "exit_code": 0, "tripwire": true},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "TRIPWIRE") {
		t.Fatalf("non-claude exit-0 >60s source=none did not surface a TRIPWIRE line (AC1):\n%s", text)
	}
	// AC2: the offending line must name CLI + agent + cycle together on one line.
	tw := lineContaining(text, "agy")
	if tw == "" {
		t.Fatalf("no tripwire line naming CLI agy:\n%s", text)
	}
	for _, want := range []string{"auditor", "6"} {
		if !strings.Contains(tw, want) {
			t.Errorf("tripwire line %q missing %q (agent/cycle not co-located with CLI — AC2)", tw, want)
		}
	}
}

// TestTokensReport_TripwireSilentOnClaudeShortAndAbort — AC3 NEGATIVE matrix.
// Three distinct false-positive vectors in one cycle — a claude-tmux baseline
// (out of scope), a non-claude success under the 60s duration threshold, and a
// non-claude quota-abort (exit 85) — must all stay silent: no TRIPWIRE line. This
// rejects an always-on implementation. All three records carry tripwire:false, the
// shape the engine writes for each non-fire condition.
func TestTokensReport_TripwireSilentOnClaudeShortAndAbort(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "8", []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 200, Output: 20}},
	})
	writeTokensLLMCalls(t, root, "8", []map[string]any{
		// claude baseline: out of tripwire scope regardless of duration/exit.
		{"agent": "builder", "phase": "build", "cli": "claude-tmux", "source": "events_result",
			"duration_ms": 90000, "exit_code": 0, "tripwire": false},
		// non-claude success UNDER the 60s threshold: too short to have burned
		// unmeasured tokens.
		{"agent": "scout", "phase": "scout", "cli": "agy", "source": "none",
			"duration_ms": 3000, "exit_code": 0, "tripwire": false},
		// non-claude quota-abort (exit 85): a failed launch, not an unmeasured
		// success.
		{"agent": "auditor", "phase": "audit", "cli": "codex", "source": "none",
			"duration_ms": 90000, "exit_code": 85, "tripwire": false},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if strings.Contains(out.String(), "TRIPWIRE") {
		t.Errorf("claude-baseline / sub-threshold / exit-85 cycle emitted a TRIPWIRE line (false positive — AC3):\n%s", out.String())
	}
}
