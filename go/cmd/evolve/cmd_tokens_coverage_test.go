package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

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
