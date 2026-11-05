package bridge

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

func (w replWaiter) closeout(state *replWaitState) (replWaitResult, int) {
	if state.completed {
		return state.result, ExitOK
	}
	if state.heldDeliverableStands() {
		fmt.Fprintf(w.deps.Stderr, "%s the session did not write %s again after the returned findings (%s); the first deliverable stands and the review gate judges it\n", w.prefix, w.cfg.Artifact, waitEndOf(state))
		return state.result, ExitOK
	}

	fmt.Fprintf(w.deps.Stderr, "%s FAIL: completion never signalled (artifact %s; %s)\n", w.prefix, w.cfg.Artifact, waitEndOf(state))
	fmt.Fprintf(w.deps.Stderr, "%s diagnostic: files present under workspace %s:\n", w.prefix, w.cfg.Workspace)
	for _, line := range listWorkspaceFiles(w.cfg.Workspace) {
		fmt.Fprintf(w.deps.Stderr, "%s   %s\n", w.prefix, line)
	}

	if state.lastVerdict.Action == ReviewPause || state.lastVerdict.Action == ReviewStop {
		_ = writeEscalationReport(w.cfg.Workspace, w.phaseName, w.cfg.Cycle, state.lastEvent, state.lastVerdict)
	}

	strippedPane := strippedForExhaustionScan(state.result.lastGoodPane, w.responder.injectedPrompt)
	warnExhaustionRegexDrift(w.deps.Stderr, w.prefix, w.launch.name, strippedPane, state.paneProfile.ExhaustedRegex)
	transient := classifyTransientPane(w.launch.name, strippedPane)
	if denial, found := guards.DenialOf(w.cfg.ProjectRoot, w.cfg.DispatchID); found && !denial.At.Before(state.lastProgressAt) {
		state.guardDenial = &denial
	}
	w.writeArtifactTimeoutMarker(state, transient)

	if state.transientShortcircuit && w.ctx.Err() == nil {
		w.deps.Sleep(transientRedispatchDelay)
	}
	return state.result, ExitArtifactTimeout
}

func waitEndOf(state *replWaitState) string {
	if state.paneLost {
		return "the pane was lost"
	}
	return fmt.Sprintf("stop-review paused after %d interval(s) of %ds", state.attempt+1, state.intervalS)
}
