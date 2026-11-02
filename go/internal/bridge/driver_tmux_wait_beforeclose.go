package bridge

import "fmt"

func (s *replWaitState) heldDeliverableStands() bool {
	return s.heldAfterCompletion && s.cancellationErr == nil && !s.paneLost && !s.transientShortcircuit && !s.submitWedged
}

func (w replWaiter) returnFindingsBeforeClose(state *replWaitState, elapsed int) bool {
	if w.deps.beforeClose == nil || state.findingsChecked {
		return false
	}
	state.findingsChecked = true
	findings := w.deps.beforeClose(w.ctx)
	if findings == "" {
		return false
	}
	base := w.deps.CaptureBaseline(w.cfg)
	base.worktree = captureWorktreeEvidenceBaseline(w.ctx, w.cfg, w.deps)
	if err := injectText(w.ctx, w.cfg, w.deps, w.launch.session, findings); err != nil {
		fmt.Fprintf(w.deps.Stderr, "%s WARN: the handoff floor findings did not enter the live session (%v); the session closes and the review gate runs as today\n", w.prefix, err)
		return false
	}
	fmt.Fprintf(w.deps.Stderr, "%s returned the handoff floor findings into the live session; the session closes after a new write of %s\n", w.prefix, w.cfg.Artifact)
	state.detector = newCompletionDetector(w.cfg.Completion, w.cfg, w.deps, w.launch, base)
	state.heldAfterCompletion = true
	state.attempt, state.intervalStartS = 0, elapsed
	return true
}
