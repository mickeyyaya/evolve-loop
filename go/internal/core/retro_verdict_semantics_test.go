package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestDecideAfterRetro_RetroPASSIsNotRecovery(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	dir := t.TempDir()
	// A plain task-level audit FAIL: deterministic floor silent, no agent floor
	// claim, no disposition — so the ladder's terminal arm is the adapter.
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 the auditor rejected this build")
	cs := CycleState{CycleID: 1577, WorkspacePath: dir}

	next, _, reason, _ := o.decideAfterRetro(cs, VerdictPASS, nil)

	if next == PhaseShip {
		t.Errorf("retro PASS routed a FAILED cycle to ship; the tree is byte-identical to the one audit rejected (reason=%q)", reason)
	}
}

func TestDecideAfterRetro_RetroPASSStillFacesTheFloor(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	dir := t.TempDir()
	// infra-systemic produces a DETERMINISTIC floor candidate.
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic", "all CLI families exhausted")
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictPASS, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("a deterministic floor candidate must halt even when the retrospective is well written; sig=%+v", sig)
	}
	if next != PhaseEnd {
		t.Errorf("next = %s, want end", next)
	}
}

func TestDecideAfterRetro_RetroPASSReachesTheSameLadderAsFAIL(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	passNext, _, passReason, passSig := o.decideAfterRetro(cs, VerdictPASS, nil)
	failNext, _, failReason, failSig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if passNext != failNext || passReason != failReason || (passSig == nil) != (failSig == nil) {
		t.Errorf("retro PASS and FAIL must reach the same disposition:\n  PASS: %s %q\n  FAIL: %s %q",
			passNext, passReason, failNext, failReason)
	}
	if passNext == PhaseShip {
		t.Error("retro PASS shipped a cycle the auditor rejected")
	}
}

func TestDecideAfterRetro_RetroFAILPathUnchanged(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic", "all CLI families exhausted")
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt || next != PhaseEnd {
		t.Fatalf("retro FAIL disposition changed; sig=%+v next=%s", sig, next)
	}
}

func TestDecideAfterRetroRouted_RetroPASSCannotDropTheFloorSignal(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic", "all CLI families exhausted")
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetroRouted(
		context.Background(), 1001, cs, 1, VerdictPASS, nil, router.RouteInput{})

	if sig == nil {
		t.Fatal("the floor signal was discarded on the retro-PASS arm; a system failure escapes ADR-0072 when the post-mortem is well written")
	}
	if !sig.Halt || next != PhaseEnd {
		t.Errorf("floor must halt: sig.Halt=%v next=%s", sig.Halt, next)
	}
}
