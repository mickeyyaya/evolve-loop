package core

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const carryoverPriorityLesson = carryover.PriorityLesson

// judgmentTeachingPhases are the phases whose FAIL verdict is a reasoned
// objection worth carrying to the next cycle.
var judgmentTeachingPhases = map[Phase]bool{
	Phase("premise-challenge"): true,
	// plan-review has no .evolve/phases/plan-review/phase.json today (it exists
	// as an agent persona + slash command); listed so the lesson works the day
	// it becomes a catalog phase.
	Phase("plan-review"):        true,
	Phase("adversarial-review"): true,
}

// recordJudgmentLesson records a judgment phase's FAIL verdict as a carryover
// todo; a no-op for any phase outside judgmentTeachingPhases.
func (o *Orchestrator) recordJudgmentLesson(ctx context.Context, cycle int, workspace string, failed Phase, state *State, diags []Diagnostic) {
	if state == nil || !judgmentTeachingPhases[failed] {
		return
	}
	if o.isAuthoritativePhase(failed) {
		return
	}
	if len(cyclestate.ErrorMessages(diags)) == 0 {
		if failure, ok := phasecontract.ReadFailureBlock(workspace, string(failed)); ok {
			for _, defect := range failure.Defects {
				diags = append(diags, Diagnostic{Severity: cyclestate.SeverityError, Message: defect})
			}
		}
	}
	summary := failureLearningSummary(cycle, failed, floorVerdictError(failed, diags))
	todoID := fmt.Sprintf("cycle-%d-judgment-%s", cycle, failed)
	expiresAt := failurelog.ComputeExpiresAt(failurelog.IntentRejected, o.now().UTC())
	o.appendCarryoverTodoDeduped(state, CarryoverTodo{
		ID: todoID, Action: summary, Priority: carryoverPriorityLesson,
		FirstSeenCycle: cycle, ExpiresAt: expiresAt,
	})
	o.writeFailureLearningState(ctx, state)
}
