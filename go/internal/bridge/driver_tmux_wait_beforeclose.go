package bridge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

var errFindingsSubmitWedged = errors.New("findings submission wedged")

type closeCheckResult struct {
	findings string
	report   func(error)
}

func (s *replWaitState) heldDeliverableStands() bool {
	return s.heldAfterCompletion && s.cancellationErr == nil && !s.paneLost && !s.transientShortcircuit && !s.submitWedged
}

func (w replWaiter) returnFindingsBeforeClose(state *replWaitState, elapsed int) bool {
	if w.deps.beforeClose == nil || state.findingsChecked {
		return false
	}
	state.findingsChecked = true
	found := w.closeCheckWithin(time.Duration(state.intervalS) * time.Second)
	if found.findings == "" {
		return false
	}
	base := w.deps.CaptureBaseline(w.cfg)
	base.worktree = captureWorktreeEvidenceBaseline(w.ctx, w.cfg, w.deps)
	err := w.submitFindings(found.findings)
	if found.report != nil {
		found.report(err)
	}
	if err != nil {
		fmt.Fprintf(w.deps.Stderr, "%s WARN: the handoff floor findings did not enter the live session (%v); the session closes and the review gate runs as today\n", w.prefix, err)
		return false
	}
	fmt.Fprintf(w.deps.Stderr, "%s returned the handoff floor findings into the live session; the session closes after a new write of %s\n", w.prefix, w.cfg.Artifact)
	state.detector = newCompletionDetector(w.cfg.Completion, w.cfg, w.deps, w.launch, base)
	state.heldAfterCompletion = true
	state.attempt, state.intervalStartS = 0, elapsed
	state.restartIdleBudget(w.recorder, w.deps.Now)
	return true
}

func (w replWaiter) submitFindings(findings string) error {
	if err := injectText(w.ctx, w.cfg, w.deps, w.launch.session, findings); err != nil {
		return err
	}
	outcome := w.verifyInjected("findings", findings)
	if outcome.Result == interaction.ResultSubmitWedged {
		return fmt.Errorf("%w (resends=%d)", errFindingsSubmitWedged, outcome.Resends)
	}
	return nil
}

func (w replWaiter) closeCheckWithin(deadline time.Duration) closeCheckResult {
	ctx, cancel := context.WithTimeout(w.ctx, deadline)
	defer cancel()
	start := w.deps.Now()
	done := make(chan closeCheckResult, 1)
	go func() {
		findings, report := w.deps.beforeClose(ctx)
		done <- closeCheckResult{findings: findings, report: report}
	}()
	select {
	case found := <-done:
		return found
	case <-ctx.Done():
		fmt.Fprintf(w.deps.Stderr, "%s WARN: the handoff floor before close passed its deadline of %s after %s; nothing is delivered and the session closes as today\n", w.prefix, deadline, w.deps.Now().Sub(start).Round(time.Millisecond))
		return closeCheckResult{}
	}
}
