package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func (o *Orchestrator) recordReviewRound(cs CycleState, phase Phase, verdict string) *cyclestate.ReviewRepair {
	if !reviewPhaseRecorded(phase, verdict) {
		return cs.ReviewRepair
	}
	req := codereview.Request{Workspace: cs.WorkspacePath, Cycle: cs.CycleID, Round: codereview.RoundFrom(cs.CompletedPhases)}
	outcome := codereview.Record(req, codereview.Settings{Repair: o.workflowConfig.CodeReviewRepair, Index: o.workflowConfig.QualityIndex})
	e := outcome.Event()
	e.RunID = o.signalRunID()
	o.signals.Emit(e)

	state := cs.ReviewRepair
	in := codereview.RoundInput{Round: req.Round, LedgerErr: outcome.Err, Config: o.workflowConfig.Convergence}
	if outcome.Err == nil {
		var doc defectledger.Doc
		doc, _, in.LedgerErr = defectledger.Read(req.Workspace)
		if in.LedgerErr == nil {
			in.Rows = doc.Entries
			state = state.WithRound(req.Round, codereview.TakeSnapshot(outcome.Scores, o.workflowConfig.QualityIndex.Thresholds, doc.Entries))
		}
	}
	in.Snapshots = state.Snapshots()
	round := codereview.RoundEvent(req, o.workflowConfig.CodeReviewRepair.Stage, codereview.DecideAfterReview(in))
	round.RunID = o.signalRunID()
	o.signals.Emit(round)
	return state
}

func (o *Orchestrator) recordReviewSkipped(cs CycleState, phase Phase) {
	if phase != Phase(codereview.PhaseName) {
		return
	}
	e := codereview.Skipped(codereview.Request{Workspace: cs.WorkspacePath, Cycle: cs.CycleID, Round: codereview.RoundFrom(cs.CompletedPhases) + 1}, codereview.SkipMalformed)
	e.RunID = o.signalRunID()
	o.signals.Emit(e)
}
