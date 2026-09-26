package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestFloorVerdictError_JoinsErrorSeverityDiagnostics(t *testing.T) {
	diags := []Diagnostic{
		{Severity: "warning", Message: "gofmt gate skipped (could not run)"},
		{Severity: "error", Message: "skill projection drift: Run `evolve skills generate`. Drifted: skills/retro/SKILL.md"},
		{Severity: "error", Message: "EGPS: red_count=2"},
	}
	err := floorVerdictError(PhaseAudit, diags)
	if err == nil {
		t.Fatal("floorVerdictError must return a non-nil error for a FAIL verdict")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "audit verdict=FAIL:") {
		t.Errorf("missing phase/verdict prefix: %q", msg)
	}
	if !strings.Contains(msg, "evolve skills generate") {
		t.Errorf("remediation must survive into the reason so the loop can act on it: %q", msg)
	}
	if !strings.Contains(msg, "red_count=2") {
		t.Errorf("all error-severity diagnostics must be joined: %q", msg)
	}
	if strings.Contains(msg, "gofmt gate skipped") {
		t.Errorf("warning-severity (fail-open) diagnostics must NOT enter the reason: %q", msg)
	}

	generic := floorVerdictError(PhaseAudit, []Diagnostic{{Severity: "warning", Message: "x"}})
	if generic.Error() != "audit verdict=FAIL" {
		t.Errorf("generic fallback = %q, want %q", generic.Error(), "audit verdict=FAIL")
	}
}

func TestRecordFailedApproachState_RecordsFloorFailToStateAndCarryover(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	o.now = func() time.Time { return time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC) }
	st := &State{}
	cs := &CycleState{WorkspacePath: t.TempDir()}
	fl := failureLearningRequest{
		Cycle:      850,
		Failed:     PhaseAudit,
		Err:        floorVerdictError(PhaseAudit, []Diagnostic{{Severity: "error", Message: "skill projection drift: Run `evolve skills generate`. Drifted: skills/retro/SKILL.md"}}),
		State:      st,
		CycleState: cs,
	}

	summary, todoID, _ := o.recordFailedApproachState(fl)

	if len(st.FailedAt) != 1 {
		t.Fatalf("FailedAt len=%d, want 1 (the floor FAIL must be recorded so the adapter/Scout can learn it)", len(st.FailedAt))
	}
	rec := st.FailedAt[0]
	if rec.Cycle != 850 {
		t.Errorf("record cycle=%d, want 850", rec.Cycle)
	}
	if rec.Verdict != VerdictFAIL {
		t.Errorf("record verdict=%q, want FAIL", rec.Verdict)
	}
	if !strings.Contains(rec.Summary, "evolve skills generate") {
		t.Errorf("record summary must carry the remediation reason, got %q", rec.Summary)
	}
	if todoID != "cycle-850-failed-audit" {
		t.Errorf("todoID=%q, want cycle-850-failed-audit", todoID)
	}
	if !strings.Contains(summary, "audit") {
		t.Errorf("summary must name the phase, got %q", summary)
	}
	if st.LastCycleNumber != 850 {
		t.Errorf("LastCycleNumber=%d, want 850", st.LastCycleNumber)
	}
	foundP0 := false
	for _, td := range st.CarryoverTodos {
		if td.ID == todoID && td.Priority == "P0" {
			foundP0 = true
		}
	}
	if !foundP0 {
		t.Errorf("a P0 carryover todo for %s must be queued so the next cycle reconsiders before retrying; got %+v", todoID, st.CarryoverTodos)
	}
}

func TestRecordFailedApproachState_DedupesCarryover(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	o.now = func() time.Time { return time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC) }
	st := &State{}
	cs := &CycleState{WorkspacePath: t.TempDir()}
	mk := func() failureLearningRequest {
		return failureLearningRequest{Cycle: 851, Failed: PhaseAudit, Err: floorVerdictError(PhaseAudit, nil), State: st, CycleState: cs}
	}
	o.recordFailedApproachState(mk())
	o.recordFailedApproachState(mk())

	todos := 0
	for _, td := range st.CarryoverTodos {
		if td.ID == "cycle-851-failed-audit" {
			todos++
		}
	}
	if todos != 1 {
		t.Errorf("carryover todo appended %d times, want 1 (deduped by id)", todos)
	}
}

func TestRecordAndBranch_AuditFAILRecordsFloorFailure(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow() // audit acquires the ship-window lease; free it + its heartbeat

	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "skill projection drift: Run `evolve skills generate`. Drifted: skills/retro/SKILL.md"}},
	}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseAudit, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if len(cr.state.FailedAt) != 1 {
		t.Fatalf("audit FAIL (err==nil) must record to state.FailedAt so the adapter/Scout learn it; got %d entries", len(cr.state.FailedAt))
	}
	if !strings.Contains(cr.state.FailedAt[0].Summary, "evolve skills generate") {
		t.Errorf("recorded reason must carry the gate remediation, got %q", cr.state.FailedAt[0].Summary)
	}
	persisted, err := cr.o.storage.ReadState(context.Background())
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if len(persisted.FailedAt) != 1 {
		t.Errorf("floor FAIL must be PERSISTED to storage (durability), not only in-memory; got %d record(s) on disk", len(persisted.FailedAt))
	}
}

func TestRecordAndBranch_AuditPASSDoesNotRecord(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()

	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseAudit, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if len(cr.state.FailedAt) != 0 {
		t.Errorf("audit PASS must NOT record a failed approach; got %d", len(cr.state.FailedAt))
	}
}

func TestRecordAndBranch_NonAuthoritativePhaseFAILDoesNotRecord(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})

	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}
	if _, err := cr.recordAndBranch(PhaseScout, dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if len(cr.state.FailedAt) != 0 {
		t.Errorf("non-authoritative phase FAIL must NOT record via the floor path; got %d", len(cr.state.FailedAt))
	}
}
