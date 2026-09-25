package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// idleNudge is the ONE decision the idle reminder's text, the operator log
// label and the interaction trigger all come from (F39, cycle 1691).
type idleNudge struct {
	msg, label, trigger string
}

// idleNudgeFor words the reminder by what the host sees. A deliverable found
// at any candidate location (artifactLocate — the resolution the completion
// poll and the admission check share) that still matches the dispatch
// baseline gets the reason a rewrite is needed: completion is the deliverable
// being written after THIS dispatch (the stale-leftover guard, right for every
// dispatch), so an agent told only to "write the deliverable" sees a report
// that is already right and stops. The remedy is bound to a re-check — the
// agent answers "is it still true for this attempt?", never "touch it" — so a
// stale verdict cannot be carried forward unexamined (architecture review M1).
// Otherwise the plain reminder.
func idleNudgeFor(cfg *Config, base artifactBaseline) idleNudge {
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

// carryForwardRemedy is how a re-checked deliverable is carried forward by its
// format (architecture review M2): a markdown report takes an appended
// attestation line; any other format (a JSON plan) is written again in full —
// a line appended to JSON breaks its parse.
func carryForwardRemedy(artifact string) string {
	if strings.EqualFold(filepath.Ext(artifact), ".md") {
		return "if it still holds, append a line recording what this attempt changed and that you re-verified it; if not, rewrite it."
	}
	return "if it still holds, write it again in full, in the same format; if not, rewrite it with this attempt's result."
}

// deliverArtifactNudge sends and verifies the one permitted idle reminder. A
// wedged submission ends the wait with its classified reason; every other
// verification result records the nudge and begins one final interval.
func (w replWaiter) deliverArtifactNudge(state *replWaitState, elapsed int) checkpointDispositionResult {
	nudge := idleNudgeFor(w.cfg, w.artifactBase)
	nudgeMsg := nudge.msg
	_ = w.deps.Tmux.SendKeys(w.ctx, w.launch.session, nudgeMsg, true)
	// Announce the nudge before verification so a re-send diagnostic cannot
	// appear before the operator is told that the nudge exists.
	fmt.Fprintf(w.deps.Stderr, "%s %s; sent one-shot nudge: %s\n", w.prefix, nudge.label, nudgeMsg)
	w.deps.Sleep(submitVerifySettle)
	nudgePane, captureErr := w.deps.Tmux.CapturePane(w.ctx, w.launch.session, w.launch.bootScrollback)
	if captureErr != nil {
		fmt.Fprintf(w.deps.Stderr, "%s submit-verify: nudge NOT verified — capture failed, input-line state unknown: %v\n", w.prefix, captureErr)
	}
	outcome := verifySubmitted(w.ctx, w.deps, w.launch, w.prefix, "nudge", nudgePane, nudgeMsg)
	recordSubmitVerify(w.recorder, w.phaseName, w.cfg.Cycle, "nudge", outcome, pasteOutcome{}) // a nudge is a SendKeys, not a paste
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
