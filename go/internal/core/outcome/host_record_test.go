package outcome

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func readSidecar(t *testing.T, path string) UsageSidecar {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s UsageSidecar
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRecorder_Record_AHostRecordNeverErasesTheDispatchedUsage(t *testing.T) {
	c, _ := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	result := &cyclestate.CycleResult{Cycle: 1757}
	var timings []phasetiming.Entry
	r.Record(result, &timings, ws, recovery.PhaseOutcome{Phase: "triage", Verdict: "PASS", CostUSD: 1.25, DurationMS: 90000, AttemptCount: 1})

	r.Record(result, &timings, ws, recovery.PhaseOutcome{Phase: "triage", Verdict: "FAIL", AttemptCount: 0, AbortReason: "triage-empty-commitment-claimable-work"})

	if s := readSidecar(t, UsageSidecarPath(ws, "triage")); s.CostUSD != 1.25 || s.Verdict != "PASS" || s.AttemptCount != 1 {
		t.Errorf("sidecar = %+v; the host's zero-attempt record keeps the dispatch's usage (cyclecost sums the sidecars)", s)
	}
	if len(timings) != 2 || timings[1].AbortReason != "triage-empty-commitment-claimable-work" || timings[1].Verdict != "FAIL" {
		t.Errorf("the host record still lands in the C1 log: %+v", timings)
	}
}

func TestRecorder_Record_AHostRecordOfAnUndispatchedPhaseStillLeavesItsSidecar(t *testing.T) {
	c, _ := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	var timings []phasetiming.Entry

	r.Record(&cyclestate.CycleResult{Cycle: 3}, &timings, ws, recovery.PhaseOutcome{Phase: "build", Verdict: "FAIL", AttemptCount: 0, AbortReason: "no runner registered for phase build"})

	if s := readSidecar(t, UsageSidecarPath(ws, "build")); s.Verdict != "FAIL" || s.AbortReason == "" {
		t.Errorf("sidecar = %+v; with nothing to keep, the record's sidecar is written as before", s)
	}
}

func TestRecorder_RecordEnding_LogsTheEndingWithoutCountingARun(t *testing.T) {
	c, _ := recordingSignals()
	r, calls := newTestRecorder(c)
	ws := t.TempDir()
	timings := []phasetiming.Entry{{Phase: "triage", Verdict: "PASS", AttemptCount: 1}}

	r.RecordEnding(&timings, recovery.PhaseOutcome{Phase: "triage", Verdict: "FAIL", AbortReason: "triage-empty-commitment-claimable-work",
		Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageScopeUnanswered}}})

	if len(timings) != 2 {
		t.Fatalf("timings = %+v", timings)
	}
	e := timings[1]
	if e.Verdict != "FAIL" || e.AbortReason == "" || e.EndedAt != "2026-09-13T01:02:03Z" || e.Archetype != "role-triage" || len(e.Diagnostics) != 1 {
		t.Errorf("the ending is a stamped C1 entry: %+v", e)
	}
	if len(*calls) != 0 {
		t.Errorf("an ending is not a phase outcome, so nothing is emitted (the cycle's seal reports how it ended): %+v", *calls)
	}
	if _, err := os.Stat(UsageSidecarPath(ws, "triage")); !os.IsNotExist(err) {
		t.Errorf("an ending dispatches nothing, so it writes no usage sidecar: %v", err)
	}
}
