package core

// retro_verdict_semantics_test.go — RED contract for what a retro verdict MEANS.
//
// `decideAfterRetro` opened with:
//
//	// retro PASS → ship; no failureadapter consultation, no floor (nothing failed).
//	if retroVerdict == VerdictPASS {
//	    return o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseShip), nil, "retro-recovered: ship", nil
//	}
//
// and TestOrchestrator_RetroPASS_RoutesToShip (2026-05-23) pinned it deliberately,
// forcing audit=FAIL, retro=PASS and asserting the tail audit→retro→ship. So this
// is a considered behaviour being changed, not an oversight being tidied. Three
// independent facts say the consideration was wrong:
//
//  1. THE COMMENT'S PREMISE IS FALSE. Retro runs only when the previous verdict is
//     FAIL/WARN (retro.go: "previous verdict != FAIL/WARN → SKIPPED"), and the
//     state machine routes only audit-FAIL to retro. "Nothing failed" is never
//     true on this path.
//
//  2. RETRO CANNOT CHANGE THE TREE. Its persona is "READ-ONLY everywhere except
//     retrospective-report.md, handoff-retrospective.json, failure-decision.json,
//     and instincts/lessons/*.yaml". So the tree ship would commit is BYTE-IDENTICAL
//     to the one the auditor rejected. Nothing happened in between that could make
//     a rejected build shippable.
//
//  3. RETRO PASS ANSWERS A DIFFERENT QUESTION. retro.go computes it as
//     "retrospective non-empty AND a failure-lesson exists" — a DELIVERABLE
//     COMPLETENESS signal. Routing consumed it as "the cycle recovered". Those are
//     different questions, and a verdict is only meaningful with the question it
//     answers.
//
// Why it looked viable for three months: ship's audit binding used to be
// satisfiable by another cycle's PASS entry (the cross-run fail-open closed by
// #503 on 2026-08-26). Once ship began failing closed with
// CodeAuditBindingVerdictFail, this route's only possible outcome became an error.
// A path whose sole outcome is a guaranteed ShipError is not a designed path.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// The core semantic pin: a retro PASS on a FAILED cycle must NOT be read as
// recovery. It must fall through to the same disposition ladder a retro FAIL
// takes — floor, then bookkeeping regrade, then audit repair, then the adapter.
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

// The ladder must be REACHED, not merely not-ship. This pins that a retro PASS
// gets the same floor evaluation a retro FAIL does — otherwise a system failure
// could escape ADR-0072 simply by the post-mortem being well written.
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

// A retro PASS must reach the same disposition ladder a retro FAIL reaches. The
// original form of this test asserted it reached ADR-0092's retro-side repair
// branch; that branch is gone — retries are now decided at the AUDIT chokepoint
// from the audit's own declared class (audit_fail_decision.go). What still matters,
// and what this pins, is that a retro PASS is not short-circuited past the ladder.
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

// A retro FAIL must behave exactly as it does today. The change narrows what a
// retro PASS means; it must not alter the FAIL path at all.
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

// THE LIVE PATH, and the sharpest form of the defect. decideAfterRetroRouted had
// its own PASS early return:
//
//	detNext, extraEnv, detReason, sig := o.decideAfterRetro(...)
//	if retroVerdict == VerdictPASS {
//	    return detNext, extraEnv, detReason, nil // PASS recovers; not a failure branch
//	}
//
// It discards `sig`. So a DETERMINISTIC ADR-0072 floor candidate — the signal the
// codebase describes as "non-bypassable" and "a broken pipeline cannot dodge it" —
// was dropped whenever the retrospective happened to be well written. A system
// failure could escape the halt by writing a good post-mortem.
//
// Unreachable in practice only because the retro gate almost never returns PASS
// (the failure-lesson artifact mismatch). Fixing that gate without this would have
// made it live — the same masked-defect pair, one layer down.
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
