package core

import (
	"context"
	"fmt"
	"os"
)

// phaseCompletionRecord is the shared completion boundary for fresh and
// resumed phase execution. Dispatch and branch selection remain path-specific;
// durable completion, floor learning, and timing use one implementation.
type phaseCompletionRecord struct {
	orchestrator *Orchestrator
	ctx          context.Context
	request      CycleRequest
	cycle        int
	phase        Phase
	response     PhaseResponse
	attempts     int
	state        *State
	cycleState   *CycleState
	result       *CycleResult
	timings      *[]phaseTimingEntry

	checkpoint              bool
	includeAuditFailReasons bool
}

func (r phaseCompletionRecord) persist() error {
	cs := r.cycleState
	cs.CompletedPhases = append(cs.CompletedPhases, string(r.phase))
	r.orchestrator.recordFinalVerdict(r.result, r.phase, r.response.Verdict,
		r.orchestrator.floorAlreadyCompleted(cs.CompletedPhases))
	cs.FinalVerdict = r.result.FinalVerdict
	if err := r.orchestrator.storage.WriteCycleState(r.ctx, *cs); err != nil {
		writeErr := fmt.Errorf("write cycle-state post-%s: %w", r.phase, err)
		r.recordOutcome(writeErr.Error())
		return writeErr
	}

	if r.checkpoint && PhaseBoundaryCheckpointer != nil {
		if err := PhaseBoundaryCheckpointer(*cs, r.request.ProjectRoot, r.orchestrator.now()); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase boundary checkpoint failed: %v\n", err)
		}
	}
	if r.response.Verdict == VerdictFAIL {
		r.orchestrator.recordJudgmentLesson(r.ctx, r.cycle, cs.WorkspacePath, r.phase, r.state, r.response.Diagnostics)
	}
	if r.response.Verdict == VerdictFAIL && r.orchestrator.isAuthoritativePhase(r.phase) {
		r.orchestrator.recordFloorVerdictFailure(r.ctx, r.request, r.cycle, r.phase, r.state, cs, r.response.Diagnostics)
		if r.includeAuditFailReasons && r.phase == PhaseAudit && len(cs.AuditFailReasons) > 0 {
			r.result.FailReasons = append(r.result.FailReasons, cs.AuditFailReasons...)
		}
	}
	r.recordOutcome("")
	return nil
}

func (r phaseCompletionRecord) recordOutcome(abortReason string) {
	r.orchestrator.recordPhaseOutcome(
		r.result,
		r.timings,
		r.cycleState.WorkspacePath,
		phaseOutcomeFrom(r.phase, r.response, r.attempts, abortReason, r.cycleState.PhaseStartedAt),
	)
}
