package signalcenter

// summary_test.go — the per-cycle view a listener keeps (design §8): counts by
// severity and kind and the last INCIDENT. Process-level events (cycle 0)
// count toward the current cycle; an event for a NEWER cycle resets the view
// (a loop process runs many cycles and keeps only the current one).

import "testing"

func TestSummary_CountsBySeverityAndKindAndKeepsTheLastIncident(t *testing.T) {
	t.Parallel()
	s := NewSummary()
	s.Observe(Event{Cycle: 7, Kind: KindPhaseOutcome, Severity: SeverityInfo, Reason: "scout PASS"})
	s.Observe(Event{Cycle: 7, Kind: KindPhaseOutcome, Severity: SeverityWarn, Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL", Reason: "triage FAIL"})
	s.Observe(Event{Cycle: 0, Kind: KindLoopWave, Severity: SeverityInfo, Reason: "process-level counts too"})
	s.Observe(Event{Cycle: 7, Kind: KindSystemFailure, Severity: SeverityIncident, Code: "ORCHESTRATOR_SYSTEM_FAILURE", Reason: "halt", Seq: 4})
	s.Observe(Event{Cycle: 7, Kind: KindSystemFailure, Severity: SeverityIncident, Code: "ORCHESTRATOR_SYSTEM_FAILURE", Reason: "halt again", Seq: 5})
	v := s.Snapshot()
	if v.Cycle != 7 || v.Total != 5 {
		t.Errorf("cycle/total = %d/%d, want 7/5: %+v", v.Cycle, v.Total, v)
	}
	if v.BySeverity[SeverityInfo] != 2 || v.BySeverity[SeverityWarn] != 1 || v.BySeverity[SeverityIncident] != 2 {
		t.Errorf("by severity: %+v", v.BySeverity)
	}
	if v.ByKind[KindPhaseOutcome] != 2 || v.ByKind[KindSystemFailure] != 2 || v.ByKind[KindLoopWave] != 1 {
		t.Errorf("by kind: %+v", v.ByKind)
	}
	if v.LastIncident == nil || v.LastIncident.Reason != "halt again" || v.LastIncident.Seq != 5 {
		t.Errorf("the LAST incident is kept: %+v", v.LastIncident)
	}
}

func TestSummary_ResetsOnANewerCycleAndIgnoresOlderOnes(t *testing.T) {
	t.Parallel()
	s := NewSummary()
	s.Observe(Event{Cycle: 7, Kind: KindPhaseOutcome, Severity: SeverityWarn, Reason: "old"})
	s.Observe(Event{Cycle: 8, Kind: KindPhaseOutcome, Severity: SeverityInfo, Reason: "new cycle"})
	s.Observe(Event{Cycle: 7, Kind: KindPhaseOutcome, Severity: SeverityIncident, Reason: "late straggler from the old cycle"})
	v := s.Snapshot()
	if v.Cycle != 8 || v.Total != 1 || v.BySeverity[SeverityWarn] != 0 || v.LastIncident != nil {
		t.Errorf("a newer cycle resets the view and older cycles are ignored: %+v", v)
	}
}

func TestSummary_SnapshotIsACopyAndZeroValueIsUsable(t *testing.T) {
	t.Parallel()
	s := NewSummary()
	first := s.Snapshot()
	s.Observe(Event{Cycle: 1, Kind: KindPhaseOutcome, Severity: SeverityInfo, Reason: "x"})
	if first.Total != 0 || s.Snapshot().Total != 1 {
		t.Error("Snapshot returns an independent copy")
	}
	var zero Summary
	if v := zero.Snapshot(); v.Total != 0 || v.BySeverity == nil || v.ByKind == nil {
		t.Errorf("a zero Summary snapshots to empty, non-nil maps: %+v", v)
	}
	inc := s.Snapshot()
	inc.LastIncident = &Event{Reason: "mutating the copy"}
	if s.Snapshot().LastIncident != nil {
		t.Error("mutating a snapshot must not reach the summary")
	}
}

func TestSummary_ZeroValueObservesLikeANewSummary(t *testing.T) {
	t.Parallel()
	var zero Summary
	zero.Observe(Event{Cycle: 0, Kind: KindLoopWave, Severity: SeverityInfo, Reason: "a process-level event is the first thing a zero value sees"})
	zero.Observe(Event{Cycle: 4, Kind: KindPhaseOutcome, Severity: SeverityWarn, Reason: "then the first cycle"})
	v := zero.Snapshot()
	if v.Cycle != 4 || v.Total != 1 || v.BySeverity[SeverityWarn] != 1 || v.ByKind[KindPhaseOutcome] != 1 {
		t.Errorf("a zero Summary lazily initialises itself on the first Observe: %+v", v)
	}
}
