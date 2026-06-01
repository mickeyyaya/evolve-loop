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

// cyclehealth_cycle188_test.go — RED contracts for cycle-188.
//
// Task 1 (stop-review-ledger-trail) READ side: checkSelfHealEvents must treat
// a kind=stop_review ledger entry with action=pause as a WARN anomaly (a phase
// the reviewer paused for investigation), while action=extend is healthy and
// emits nothing.
//
// Task 2 (per-phase-latency-ceiling): checkPhaseLatency must honour a per-phase
// ceiling override EVOLVE_<PHASE_UPPER>_LATENCY_CEILING_S (phase normalized via
// ToUpper + "-"→"_"), falling back to the global ceiling when unset/invalid.

// --- Task 1 helpers ---------------------------------------------------------

// writeStopReviewLedger seeds a valid 3-role ledger plus one stop_review entry
// carrying an `action` field. The action lives in raw JSON — the ledgerEntry
// struct does not model it yet — so this test compiles BEFORE the Builder adds
// the field, giving a clean assertion-RED (not a compile error) on the
// currently-missing stop_review handling in checkSelfHealEvents.
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

// TestCheck_SelfHealEvents_StopReviewPause_Warn — AC4: a stop_review/pause
// ledger entry surfaces exactly one self_heal_events WARN (never fatal — a
// paused-then-recovered cycle must still report), naming the paused phase.
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

// TestCheck_SelfHealEvents_StopReviewExtend_NoAnomaly — AC4 anti-no-op: a
// stop_review/extend entry is a HEALTHY decision (the reviewer judged the agent
// still working) and must emit ZERO self_heal_events anomalies. An
// implementation that warned on every stop_review regardless of action fails
// this.
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

// --- Task 2 helpers ---------------------------------------------------------

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

// TestCheck_PhaseLatency_PerPhaseOverride_Warn — AC1 + AC3: a scout that took
// 200s is UNDER the 900s global default (no WARN), but a per-phase override of
// 120s makes it slow → WARN naming scout.
func TestCheck_PhaseLatency_PerPhaseOverride_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	t.Setenv("EVOLVE_SCOUT_LATENCY_CEILING_S", "120")
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	got := phaseLatencyAnomalies(r)
	if len(got) != 1 {
		t.Fatalf("phase_latency count=%d, want 1 (scout over per-phase ceiling); anomalies=%+v", len(got), r.Anomalies)
	}
	if got[0].Severity != SeverityWarn {
		t.Errorf("severity=%s, want warn", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, "scout") {
		t.Errorf("expected 'scout' in message; got %q", got[0].Message)
	}
}

// TestCheck_PhaseLatency_NoOverride_UsesGlobal_NoWarn — AC1 fallback + AC3
// anti-no-op: the SAME 200s scout, with NO per-phase override, stays under the
// 900s global default → no WARN. An implementation that always warned, or that
// dropped the global fallback, fails this.
func TestCheck_PhaseLatency_NoOverride_UsesGlobal_NoWarn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	// No EVOLVE_SCOUT_LATENCY_CEILING_S set; global default 900s applies.
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseLatencyAnomalies(r); len(got) != 0 {
		t.Errorf("200s scout under 900s global must not warn, got %d; %+v", len(got), got)
	}
}

// TestCheck_PhaseLatency_BuildPlannerNormalization_Warn — AC2 + AC4: a hyphenated
// phase name "build-planner" must normalize to EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S
// (ToUpper + "-"→"_"). An implementation that forgets the "-"→"_" replacement
// would look up the invalid env name EVOLVE_BUILD-PLANNER_..., miss the override,
// fall back to 900s, and NOT warn — failing this test.
func TestCheck_PhaseLatency_BuildPlannerNormalization_Warn(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "build-planner", DurationMS: 200000, Verdict: "PASS"}})
	t.Setenv("EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S", "100")
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	got := phaseLatencyAnomalies(r)
	if len(got) != 1 {
		t.Fatalf("phase_latency count=%d, want 1 (build-planner over normalized ceiling); anomalies=%+v", len(got), r.Anomalies)
	}
	if !strings.Contains(got[0].Message, "build-planner") {
		t.Errorf("expected 'build-planner' in message; got %q", got[0].Message)
	}
}

// TestCheck_PhaseLatency_InvalidOverride_FallsBackToGlobal — AC1: a malformed
// per-phase override ("abc") is ignored and the global ceiling applies, so a
// 200s scout under 900s global does NOT warn.
func TestCheck_PhaseLatency_InvalidOverride_FallsBackToGlobal(t *testing.T) {
	ws := freshWorkspace(t, 1)
	writePhaseTiming(t, ws, []phaseTimingEntry{{Phase: "scout", DurationMS: 200000, Verdict: "PASS"}})
	t.Setenv("EVOLVE_SCOUT_LATENCY_CEILING_S", "abc") // invalid → ignored
	r, err := Check(Options{Cycle: 1, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseLatencyAnomalies(r); len(got) != 0 {
		t.Errorf("invalid override must fall back to 900s global (no warn for 200s), got %d; %+v", len(got), got)
	}
}
