package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Class fix (skills-drift storm, cycles 836/838/841/843/849): a FLOOR phase
// (audit) that returns a FAIL verdict with NO dispatch error — audit's in-process
// CI-parity gates override a narrative PASS to FAIL — was never fed to
// failure-learning, so state.FailedAt stayed empty and the failure-adapter +
// Scout could not learn the recurrence. These tests pin the new success-path
// learning: the synthesized reason and the FailedAt/carryover record.

// TestFloorVerdictError_JoinsErrorSeverityDiagnostics: the synthesized error
// names WHY the phase failed (the remediation-bearing gate messages), joining
// ONLY error-severity diagnostics so a fail-open WARN never pollutes the reason.
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

	// No error-severity diagnostics → a generic, still-non-nil reason.
	generic := floorVerdictError(PhaseAudit, []Diagnostic{{Severity: "warning", Message: "x"}})
	if generic.Error() != "audit verdict=FAIL" {
		t.Errorf("generic fallback = %q, want %q", generic.Error(), "audit verdict=FAIL")
	}
}

// TestRecordFailedApproachState_RecordsFloorFailToStateAndCarryover: the extracted
// record-only core appends a FailedRecord (carrying the reason) to state.FailedAt,
// dedupes a P0 carryover todo, stamps LastCycleNumber, and returns the summary +
// todo id — WITHOUT running retro. This is the signal the failure-adapter + Scout
// read; before the fix it was never written on the success path.
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

// TestRecordFailedApproachState_DedupesCarryover: a second identical failure in the
// same cycle must not double-append the carryover todo (idempotent per todo id),
// matching the error-path recordFailureLearning contract.
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

// The wiring tests below pin the ACTUAL trigger point — the guard in
// recordAndBranch that feeds a floor-phase FAIL verdict (returned with NO
// dispatch error) into failure-learning. This is the exact site the skills-drift
// storm re-derived forever because nothing recorded audit's err==nil gate-FAIL.
// A driftCatalog (no catalog) keeps recordAndBranch on its literal defaults;
// the floor is the router default {tdd,build,audit}, so audit is authoritative.

// TestRecordAndBranch_AuditFAILRecordsFloorFailure is the faithful regression:
// audit returns FAIL with error-severity diagnostics (the skills-drift gate) and
// NO dispatch error, so recordAndBranch's success path — not an error path —
// must record it to state.FailedAt with the remediation carried through.
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
	// Durability: the record must reach STORAGE, not only in-memory state — the
	// live loop's abort branches return before finalizeCycle persists. Pins the
	// writeFailureLearningState call in recordFloorVerdictFailure; without it this
	// asserts against the persisted copy and fails.
	persisted, err := cr.o.storage.ReadState(context.Background())
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if len(persisted.FailedAt) != 1 {
		t.Errorf("floor FAIL must be PERSISTED to storage (durability), not only in-memory; got %d record(s) on disk", len(persisted.FailedAt))
	}
}

// TestRecordAndBranch_AuditPASSDoesNotRecord: the SAME authoritative phase, when
// it PASSes, must add no failed-approach record — the guard keys on the FAIL
// verdict, not on the phase being a floor phase.
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

// TestRecordAndBranch_NonAuthoritativePhaseFAILDoesNotRecord: a non-floor phase
// (scout) FAIL is not authoritative, so the floor-learning guard must skip it —
// scout failures are handled by the normal flow, not this record-only path.
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
