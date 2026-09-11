package bridge

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// checkpointDispositionResult tells the wait coordinator whether the verdict
// starts another interval or ends the completion wait. Stop is the zero value
// so an unrecognized review action fails closed.
type checkpointDispositionResult uint8

const (
	checkpointStopWaiting checkpointDispositionResult = iota
	checkpointContinueWaiting
)

// applyCheckpointDisposition translates the published review verdict into the
// next wait transition. Checkpoint review and its callback have already run;
// this method owns only extension accounting and the optional idle nudge.
func (w replWaiter) applyCheckpointDisposition(state *replWaitState, elapsed int) checkpointDispositionResult {
	if state.lastVerdict.Action == ReviewExtend {
		state.attempt++
		state.intervalStartS = elapsed
		return checkpointContinueWaiting
	}
	if state.lastVerdict.Action != ReviewPause {
		return checkpointStopWaiting
	}
	switch state.reviewer.(type) {
	case deterministicReviewer, *deterministicReviewer:
	default:
		return checkpointStopWaiting
	}
	if state.livenessCenter.Busy(w.launch.session) || state.nudgeSent {
		return checkpointStopWaiting
	}
	return w.deliverArtifactNudge(state, elapsed)
}

// deliverArtifactNudge sends and verifies the one permitted idle reminder. A
// wedged submission ends the wait with its classified reason; every other
// verification result records the nudge and begins one final interval.
func (w replWaiter) deliverArtifactNudge(state *replWaitState, elapsed int) checkpointDispositionResult {
	nudgeMsg := fmt.Sprintf("Please write the deliverable to %s to complete the phase.", w.cfg.Artifact)
	_ = w.deps.Tmux.SendKeys(w.ctx, w.launch.session, nudgeMsg, true)
	// Announce the nudge before verification so a re-send diagnostic cannot
	// appear before the operator is told that the nudge exists.
	fmt.Fprintf(w.deps.Stderr, "%s idle with missing artifact; sent one-shot nudge: %s\n", w.prefix, nudgeMsg)
	w.deps.Sleep(submitVerifySettle)
	nudgePane, captureErr := w.deps.Tmux.CapturePane(w.ctx, w.launch.session, w.launch.bootScrollback)
	if captureErr != nil {
		fmt.Fprintf(w.deps.Stderr, "%s submit-verify: nudge NOT verified — capture failed, input-line state unknown: %v\n", w.prefix, captureErr)
	}
	outcome := verifySubmitted(w.ctx, w.deps, w.launch, w.prefix, "nudge", nudgePane, nudgeMsg)
	recordSubmitVerify(w.recorder, w.phaseName, w.cfg.Cycle, "nudge", outcome)
	if outcome.Result == interaction.ResultSubmitWedged {
		state.submitWedged = true
		state.lastVerdict.Reason = fmt.Sprintf("nudge %s (resends=%d)", outcome.Result, outcome.Resends)
		return checkpointStopWaiting
	}
	state.nudgeSent = true
	state.nudgeEvent = &interaction.Event{
		Kind:    interaction.KindNudge,
		Phase:   w.phaseName,
		Cycle:   w.cfg.Cycle,
		Trigger: "idle_no_artifact",
		Payload: nudgeMsg,
	}
	state.nudgeAt = w.deps.Now()
	state.intervalStartS = elapsed
	state.attempt++
	return checkpointContinueWaiting
}
