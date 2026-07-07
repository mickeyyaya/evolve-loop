package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func writeTokensTimingFixture(t *testing.T, root string, cycle string, entries []phasetiming.Entry) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTokensReport_RanksPhasesByInputTokens is the S7 RED test: across the
// walked cycles, phases must rank highest-InputTokens-first in the report,
// regardless of the order phases appear within any one cycle's log.
func TestTokensReport_RanksPhasesByInputTokens(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "1", []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 500, Output: 50}},
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 9000, Output: 800}},
	})
	writeTokensTimingFixture(t, root, "2", []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 1000, Output: 100}},
		{Phase: "audit", Verdict: "FAIL", Tokens: cyclestate.TokenUsage{Input: 2000, Output: 200}},
	})

	var out, errb bytes.Buffer
	code := runTokensReport([]string{"--project-root", root, "--last", "8", "--json"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	var report TokensReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, out.String())
	}
	var top PhaseTokenTotal
	if len(report.Phases) > 0 {
		top = report.Phases[0]
	}
	if len(report.Phases) != 3 {
		t.Fatalf("phases=%d want 3: %+v", len(report.Phases), report.Phases)
	}
	// build (10000) > audit (2000) > scout (500)
	wantOrder := []string{"build", "audit", "scout"}
	for i, want := range wantOrder {
		if report.Phases[i].Phase != want {
			t.Fatalf("phases[%d]=%q want %q (ranking order): %+v", i, report.Phases[i].Phase, want, report.Phases)
		}
	}
	if top.Phase != "build" || top.Tokens.Input != 10000 {
		t.Fatalf("top=%+v want phase=build input=10000", top)
	}
	if top.CycleCount != 2 {
		t.Fatalf("build cycle_count=%d want 2 (ran in both cycles)", top.CycleCount)
	}
	if report.WastedTokens.Input != 2000 {
		t.Fatalf("wasted input=%d want 2000 (audit FAIL)", report.WastedTokens.Input)
	}
}

// TestRunTokensReport_TableRendersPhasesAndTotals covers the human-readable
// path (no --json): the table must name every phase and the total line.
func TestRunTokensReport_TableRendersPhasesAndTotals(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "9", []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 100, Output: 10, CacheRead: 5}},
	})

	var out, errb bytes.Buffer
	code := runTokensReport([]string{"--project-root", root}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"scout", "Total:", "Cache-hit ratio"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

// TestRunTokensReport_NoLogsErrors covers the missing-evidence error path.
func TestRunTokensReport_NoLogsErrors(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	code := runTokensReport([]string{"--project-root", root}, &out, &errb)
	if code == 0 {
		t.Fatalf("exit=0, want non-zero when no timing logs exist")
	}
}
