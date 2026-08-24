package core

// judgment_lesson.go — a JUDGMENT phase's FAIL verdict teaches without halting.
//
// A judgment phase (premise-challenge, plan-review, adversarial-review) returns
// FAIL as a VERDICT, with err == nil. Neither learning path catches that shape:
// recordFloorVerdictFailure fires only for isAuthoritativePhase() (the resolved
// ship floor plus ship), and recordFailureLearning early-returns unless
// fl.Err != nil. So the objection left no trace in state, the next cycle's Scout
// had no memory of it, and the same falsified premise was re-derived.
//
// The lesson is recorded as a carryover todo ONLY — deliberately never a
// FailedRecord in state.FailedAt. That array feeds failureadapter.Decide, which
// carries two halt vectors: tailInfraTransientStreak breaks on any foreign
// class, and sameClassStreak manufactures a streak from consecutive same-class
// records. A phase whose entire job is to object must not be able to halt the
// batch BY objecting.

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// carryoverPriorityLesson is the priority for a judgment lesson. Deliberately
// NOT carryoverPriorityBlocking: a lesson is advice for the next cycle's planner,
// and filed at P0 it would outrank the cycle's actual blocking work and distort
// triage — the planner reads priority, not provenance.
const carryoverPriorityLesson = "P1"

// judgmentTeachingPhases are the phases whose FAIL verdict is a REASONED
// OBJECTION worth carrying to the next cycle.
//
// Deliberately an EXPLICIT set, not a derived one. The obvious derivation —
// `remediationDenied[p] && !isAuthoritativePhase(p)` — silently sweeps in
// PhaseRetro and PhaseDebugger, which are CONTROL phases, not judgment phases.
// Retro is the mechanism that WRITES lessons, so teaching from a retro FAIL
// would recurse into its own output; recordFailureLearning already excludes
// PhaseRetro explicitly, and a derived set calling in here would route around
// that guard. Audit is absent because it is authoritative — its FAIL is already
// recorded, with a FailedRecord, by recordFloorVerdictFailure.
//
// Adding a phase here is a deliberate act: it must be a phase that RENDERS
// JUDGMENT on the work, never one that controls the loop.
var judgmentTeachingPhases = map[Phase]bool{
	Phase("premise-challenge"): true,
	// plan-review has no .evolve/phases/plan-review/phase.json today (it exists as
	// an agent persona + slash command); listed deliberately so the lesson works
	// the day it becomes a catalog phase, and because it is unambiguously judgment.
	Phase("plan-review"):        true,
	Phase("adversarial-review"): true,
}

// recordJudgmentLesson records a judgment phase's FAIL verdict as a carryover
// todo and persists it. No-op for any phase outside judgmentTeachingPhases, so
// both call sites (recordAndBranch and the resume path) can call it
// unconditionally on a FAIL verdict.
//
// Persists immediately for the same reason recordFloorVerdictFailure does: this
// runs mid-loop, and several abort branches of both call sites return WITHOUT
// reaching finalizeCycle's persist. An in-memory-only append would be lost
// exactly when the cycle dies — which is the case this exists for.
func (o *Orchestrator) recordJudgmentLesson(ctx context.Context, cycle int, workspace string, failed Phase, state *State, diags []Diagnostic) {
	if state == nil || !judgmentTeachingPhases[failed] {
		return
	}
	// An operator MAY configure a judgment phase into policy.ship_floor, which
	// makes it authoritative — and then recordFloorVerdictFailure fires for the
	// same FAIL and records a real FailedRecord. Yielding to it keeps design
	// constraint #1 honest (exactly one recorder per FAIL) instead of leaving a
	// P1 "advice" todo shadowing a halt-capable failure record. Fingerprint
	// dedupe would otherwise mask the promotion: both paths synthesize the SAME
	// summary, so the floor path's P0 todo would collapse into this P1 one.
	if o.isAuthoritativePhase(failed) {
		return
	}
	if len(errorSeverityMessages(diags)) == 0 {
		if failure, ok := phasecontract.ReadFailureBlock(workspace, string(failed)); ok {
			for _, defect := range failure.Defects {
				diags = append(diags, Diagnostic{Severity: "error", Message: defect})
			}
		}
	}
	// Reuses the floor path's diagnostic synthesis so the todo names WHY the
	// judgment went against the work, not merely THAT it did — a todo reading
	// "premise-challenge failed" teaches nothing.
	summary := failureLearningSummary(cycle, failed, floorVerdictError(failed, diags))
	todoID := fmt.Sprintf("cycle-%d-judgment-%s", cycle, failed)
	// IntentRejected ("effectively never" ages out) is the honest classification:
	// this phase REJECTED the premise, and a falsified premise does not become
	// true again with time. Deliberately NOT the failure path's default string —
	// that is outside the taxonomy, so it normalizes to UnknownClassification and
	// lands in the 1-day fallback bucket, which would expire the lesson across any
	// multi-day loop pause and silently reproduce the very "Scout has no memory"
	// defect this exists to fix. The classification rides ONLY the todo's TTL;
	// carryover todos carry no classification field, so no failure-adapter path
	// (which reads state.FailedAt) can see it.
	expiresAt := failurelog.ComputeExpiresAt(failurelog.IntentRejected, o.now().UTC())
	appendCarryoverTodoDeduped(state, CarryoverTodo{
		ID: todoID, Action: summary, Priority: carryoverPriorityLesson,
		FirstSeenCycle: cycle, ExpiresAt: expiresAt,
	})
	o.writeFailureLearningState(ctx, state)
}
