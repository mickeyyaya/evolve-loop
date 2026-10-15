package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type checkpointDispositionResult uint8

const (
	checkpointStopWaiting checkpointDispositionResult = iota
	checkpointContinueWaiting
)

func (w replWaiter) applyCheckpointDisposition(state *replWaitState, elapsed int) checkpointDispositionResult {
	if state.lastVerdict.Action == ReviewExtend {
		state.attempt++
		state.intervalStartS = elapsed
		return checkpointContinueWaiting
	}
	if state.lastVerdict.Action != ReviewPause {
		return checkpointStopWaiting
	}
	if !state.lastEvent.Busy && w.completeOnIdle(state) {
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

func (w replWaiter) completeOnIdle(state *replWaitState) bool {
	completer, ok := state.detector.(idleCompleter)
	if !ok {
		return false
	}
	ready, evidence, note, err := completer.completeOnIdle(w.ctx)
	if err != nil && state.observeDetector(err) {
		fmt.Fprintf(w.deps.Stderr, "%s WARN: completion detector: %v\n", w.prefix, err)
	}
	if !ready {
		return false
	}
	state.completed = true
	fmt.Fprintf(w.deps.Stderr, "%s %s\n", w.prefix, note)
	w.deps.Signals.Emit(worktreeEvidenceEvent(w.cfg, evidence, note))
	return true
}

type idleNudge struct {
	msg, label, trigger string
}

func idleNudgeFor(cfg *Config, base dispatchBaseline) idleNudge {
	if path, found := artifactLocate(cfg); found {
		if fi, err := os.Lstat(path); err == nil && base.matches(path, fi) {
			return idleNudge{
				msg:     fmt.Sprintf("The deliverable at %s is from an earlier attempt and was not rewritten during this one; the host completes the phase only when the deliverable is written to %s after this dispatch. Re-check it against this attempt's work: %s", path, cfg.Artifact, carryForwardRemedy(cfg.Artifact)),
				label:   "idle with an unrewritten deliverable",
				trigger: "idle_unrewritten_deliverable",
			}
		}
	}
	return idleNudge{
		msg:     fmt.Sprintf("Please write the deliverable to %s to complete the phase.", cfg.Artifact),
		label:   "idle with missing artifact",
		trigger: "idle_no_artifact",
	}
}

func carryForwardRemedy(artifact string) string {
	if strings.EqualFold(filepath.Ext(artifact), ".md") {
		return "if it still holds, append a line recording what this attempt changed and that you re-verified it; if not, rewrite it."
	}
	return "if it still holds, write it again in full, in the same format; if not, rewrite it with this attempt's result."
}

func (w replWaiter) deliverArtifactNudge(state *replWaitState, elapsed int) checkpointDispositionResult {
	nudge := idleNudgeFor(w.cfg, w.dispatchBase)
	nudgeMsg := nudge.msg
	_ = w.deps.Tmux.SendKeys(w.ctx, w.launch.session, nudgeMsg, true)
	fmt.Fprintf(w.deps.Stderr, "%s %s; sent one-shot nudge: %s\n", w.prefix, nudge.label, nudgeMsg)
	w.deps.Sleep(submitVerifySettle)
	nudgePane, captureErr := w.deps.Tmux.CapturePane(w.ctx, w.launch.session, w.launch.bootScrollback)
	if captureErr != nil {
		fmt.Fprintf(w.deps.Stderr, "%s submit-verify: nudge NOT verified — capture failed, input-line state unknown: %v\n", w.prefix, captureErr)
	}
	outcome := verifySubmitted(w.ctx, w.deps, w.launch, w.prefix, "nudge", nudgePane, nudgeMsg)
	recordSubmitVerify(w.recorder, w.phaseName, w.cfg.Cycle, "nudge", outcome, pasteOutcome{})
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
		Trigger: nudge.trigger,
		Payload: nudgeMsg,
	}
	state.nudgeAt = w.deps.Now()
	state.intervalStartS = elapsed
	state.attempt++
	return checkpointContinueWaiting
}
