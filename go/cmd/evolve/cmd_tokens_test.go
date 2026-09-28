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

func TestRunTokensReport_NoLogsErrors(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	code := runTokensReport([]string{"--project-root", root}, &out, &errb)
	if code == 0 {
		t.Fatalf("exit=0, want non-zero when no timing logs exist")
	}
}

// writeTokensLLMCalls writes an llm-calls.ndjson fixture (one JSON object per
// line) into the given cycle's run dir — the same on-disk shape
// engine.recordTokenUsage emits. Each record is a map so a test sets only the
// fields it asserts on.
func writeTokensLLMCalls(t *testing.T, root, cycle string, records []map[string]any) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(ws, "llm-calls.ndjson"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// lineContaining returns the first line of text that contains needle, or "".
func lineContaining(text, needle string) string {
	for _, ln := range strings.Split(text, "\n") {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

func TestRunTokensReport_SurfacesTripwireInTextAndJSON(t *testing.T) {
	root := t.TempDir()
	// Cycle 5 is discovered via its phase-timing log; duration 90000 / exit 0
	// deliberately carry no stray "5" digit so the cycle-number assertion is real.
	writeTokensTimingFixture(t, root, "5", []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 100, Output: 10}},
	})
	writeTokensLLMCalls(t, root, "5", []map[string]any{
		{"agent": "builder", "phase": "build", "cli": "codex", "source": "none",
			"duration_ms": 90000, "exit_code": 0, "tripwire": true},
		{"agent": "scout", "phase": "scout", "cli": "claude-tmux", "source": "events_result",
			"duration_ms": 3000, "exit_code": 0, "tripwire": false},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "TRIPWIRE") {
		t.Errorf("plain-text output missing TRIPWIRE section:\n%s", text)
	}
	tw := lineContaining(text, "codex")
	if tw == "" {
		t.Errorf("no tripwire line naming CLI codex:\n%s", text)
	}
	for _, want := range []string{"builder", "5"} {
		if !strings.Contains(tw, want) {
			t.Errorf("tripwire line %q missing %q (agent/cycle)", tw, want)
		}
	}
	if strings.Contains(text, "claude-tmux") {
		t.Errorf("non-tripwire claude record surfaced as tripwire (false positive):\n%s", text)
	}

	var jout, jerr bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root, "--json"}, &jout, &jerr); code != 0 {
		t.Fatalf("json exit=%d, stderr=%s", code, jerr.String())
	}
	var report TokensReport
	if err := json.Unmarshal(jout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report json: %v\n%s", err, jout.String())
	}
	if report.TripwireCount != 1 || len(report.Tripwires) != 1 {
		t.Fatalf("json count=%d records=%d, want exactly one tripwire:\n%s", report.TripwireCount, len(report.Tripwires), jout.String())
	}
	want := TripwireEvent{Cycle: 5, CLI: "codex", Agent: "builder", Phase: "build", DurationMS: 90000, ExitCode: 0}
	if report.Tripwires[0] != want {
		t.Errorf("json tripwire=%+v, want %+v", report.Tripwires[0], want)
	}
}

func TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "7", []phasetiming.Entry{})
	writeTokensLLMCalls(t, root, "7", []map[string]any{
		{"agent": "auditor", "phase": "audit", "cli": "agy", "source": "none",
			"duration_ms": 120000, "exit_code": 0, "tripwire": true},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "TRIPWIRE") {
		t.Errorf("tripwire dropped when phases empty (render-order regression):\n%s", text)
	}
	if !strings.Contains(text, "agy") {
		t.Errorf("tripwire line missing CLI agy when phases empty:\n%s", text)
	}
}

func TestRunTokensReport_ZeroTripwireStaysQuiet(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "3", []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 200, Output: 20}},
	})
	writeTokensLLMCalls(t, root, "3", []map[string]any{
		{"agent": "builder", "phase": "build", "cli": "claude-tmux", "source": "events_result",
			"duration_ms": 90000, "exit_code": 0, "tripwire": false},
		{"agent": "builder", "phase": "build", "cli": "codex", "source": "none",
			"duration_ms": 2000, "exit_code": 85, "tripwire": false},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if strings.Contains(out.String(), "TRIPWIRE") {
		t.Errorf("zero-tripwire cycle emitted a TRIPWIRE line (false positive):\n%s", out.String())
	}
}

func TestRunTokensReport_SanitizesTripwireControlBytes(t *testing.T) {
	root := t.TempDir()
	writeTokensTimingFixture(t, root, "4", []phasetiming.Entry{
		{Phase: "build", Verdict: "PASS", Tokens: cyclestate.TokenUsage{Input: 100, Output: 10}},
	})
	writeTokensLLMCalls(t, root, "4", []map[string]any{
		{"agent": "buil\x1b[2Kder", "phase": "au\x1b[31mdit", "cli": "co\x1bdex", "source": "none",
			"duration_ms": 90000, "exit_code": 0, "tripwire": true},
	})

	var out, errb bytes.Buffer
	if code := runTokensReport([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "TRIPWIRE") {
		t.Errorf("tripwire not surfaced at all:\n%q", text)
	}
	if strings.ContainsRune(text, '\x1b') {
		t.Errorf("raw ESC (0x1b) control byte passed through to TTY (F1 injection):\n%q", text)
	}
}
