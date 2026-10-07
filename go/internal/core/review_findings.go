package core

import "github.com/mickeyyaya/evolve-loop/go/internal/codereview"

func (o *Orchestrator) recordReviewFindings(cs CycleState, phase Phase, verdict string) {
	if phase != Phase(codereview.PhaseName) || verdict == VerdictFAIL || verdict == VerdictSKIPPED {
		return
	}
	req := codereview.Request{Workspace: cs.WorkspacePath, Cycle: cs.CycleID, Round: codereview.RoundFrom(cs.CompletedPhases)}
	e := codereview.Record(req, codereview.Settings{Repair: o.workflowConfig.CodeReviewRepair, Index: o.workflowConfig.QualityIndex}).Event()
	e.RunID = o.signalRunID()
	o.signals.Emit(e)
}

func (o *Orchestrator) recordReviewSkipped(cs CycleState, phase Phase) {
	if phase != Phase(codereview.PhaseName) {
		return
	}
	e := codereview.Skipped(codereview.Request{Workspace: cs.WorkspacePath, Cycle: cs.CycleID, Round: codereview.RoundFrom(cs.CompletedPhases) + 1}, codereview.SkipMalformed)
	e.RunID = o.signalRunID()
	o.signals.Emit(e)
}
