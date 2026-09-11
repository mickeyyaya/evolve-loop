package bridge

import "fmt"

// closeout owns the ordered terminal effects for a completion wait. It does no
// polling or pane capture; the coordinator has already selected its terminal
// state before calling it.
func (w replWaiter) closeout(state *replWaitState) (replWaitResult, int) {
	if state.completed {
		return state.result, ExitOK
	}

	fmt.Fprintf(w.deps.Stderr, "%s FAIL: completion never signalled (artifact %s; stop-review paused after %d interval(s) of %ds)\n",
		w.prefix, w.cfg.Artifact, state.attempt+1, state.intervalS)
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
	w.writeArtifactTimeoutMarker(state, transient)

	if state.transientShortcircuit && w.ctx.Err() == nil {
		w.deps.Sleep(transientRedispatchDelay)
	}
	return state.result, ExitArtifactTimeout
}
