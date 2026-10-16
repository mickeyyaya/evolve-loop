package cyclehealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeStopReviewLedger(t *testing.T, ws string, cycle int, action string) {
	t.Helper()
	base := []ledgerEntry{
		{Cycle: cycle, Role: "scout", Phase: "scout", Timestamp: 1000, Token: "tok-s", EntryHash: "h1"},
		{Cycle: cycle, Role: "builder", Phase: "build", Timestamp: 1100, Token: "tok-b", PrevHash: "h1", EntryHash: "h2"},
		{Cycle: cycle, Role: "auditor", Phase: "audit", Timestamp: 1200, Token: "tok-a", PrevHash: "h2", EntryHash: "h3"},
	}
	var lines []string
	for _, e := range base {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	lines = append(lines, fmt.Sprintf(
		`{"cycle":%d,"role":"build","phase":"build","timestamp":1300,"kind":"stop_review","action":%q}`,
		cycle, action))
	if err := os.WriteFile(filepath.Join(ws, "ledger.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheck_SelfHealEvents_StopReviewPause_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writeStopReviewLedger(t, ws, 1, "pause")
	r, err := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	got := selfHealAnomalies(r)
	if len(got) != 1 {
		t.Fatalf("self_heal_events count=%d, want 1 for stop_review/pause; anomalies=%+v", len(got), r.Anomalies)
	}
	if got[0].Severity != SeverityWarn {
		t.Errorf("stop_review/pause severity=%s, want warn (never fatal)", got[0].Severity)
	}
	if r.OverallFatal {
		t.Errorf("stop_review/pause must not be fatal; OverallFatal=true, anomalies=%+v", r.Anomalies)
	}
	if !strings.Contains(got[0].Message, "build") {
		t.Errorf("expected paused phase 'build' in message; got %q", got[0].Message)
	}
}

func TestCheck_SelfHealEvents_StopReviewExtend_NoAnomaly(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writeStopReviewLedger(t, ws, 1, "extend")
	r, err := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	if got := selfHealAnomalies(r); len(got) != 0 {
		t.Errorf("stop_review/extend is healthy and must emit 0 anomalies, got %d; %+v", len(got), got)
	}
}

func writePhaseTiming(t *testing.T, ws string, entries []phaseTimingEntry) {
	t.Helper()
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, "phase-timing.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func phaseLatencyAnomalies(r Report) []Anomaly {
	var out []Anomaly
	for _, a := range r.Anomalies {
		if a.Signal == "phase_latency" {
			out = append(out, a)
		}
	}
	return out
}

func TestCheck_PhaseLatency_ConfiguredCeiling_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	r, err := Check(Options{Cycle: 1, Workspace: ws, PhaseLatencyCeilingS: 120})
	if err != nil {
		t.Fatal(err)
	}
	got := phaseLatencyAnomalies(r)
	if len(got) != 1 {
		t.Fatalf("phase_latency count=%d, want 1 (scout over configured ceiling); anomalies=%+v", len(got), r.Anomalies)
	}
	if got[0].Severity != SeverityWarn {
		t.Errorf("severity=%s, want warn", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, "scout") {
		t.Errorf("expected 'scout' in message; got %q", got[0].Message)
	}
}

func TestCheck_PhaseLatency_DefaultCeiling_NoWarn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseLatencyAnomalies(r); len(got) != 0 {
		t.Errorf("200s scout under 900s global must not warn, got %d; %+v", len(got), got)
	}
}

func TestCheck_PhaseLatency_ConfiguredCeilingNamesPhase(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "build-planner", DurationMS: 200000, Verdict: "PASS"}})
	r, err := Check(Options{Cycle: 1, Workspace: ws, PhaseLatencyCeilingS: 100})
	if err != nil {
		t.Fatal(err)
	}
	got := phaseLatencyAnomalies(r)
	if len(got) != 1 {
		t.Fatalf("phase_latency count=%d, want 1 (build-planner over configured ceiling); anomalies=%+v", len(got), r.Anomalies)
	}
	if !strings.Contains(got[0].Message, "build-planner") {
		t.Errorf("expected 'build-planner' in message; got %q", got[0].Message)
	}
}

func TestCheck_PhaseLatency_NonPositiveCeilingFallsBackToDefault(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	r, err := Check(Options{Cycle: 1, Workspace: ws, PhaseLatencyCeilingS: -1})
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseLatencyAnomalies(r); len(got) != 0 {
		t.Errorf("non-positive ceiling must fall back to 900s default (no warn for 200s), got %d; %+v", len(got), got)
	}
}
