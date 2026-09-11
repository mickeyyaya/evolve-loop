package bridge

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// checkpointReviewResult tells the wait coordinator whether checkpoint
// adjudication completed normally or selected the cross-CLI failover path.
type checkpointReviewResult uint8

const (
	checkpointReviewed checkpointReviewResult = iota
	checkpointFailover
)

// reviewCheckpoint retains the latest pane evidence, updates liveness and
// exhaustion gates, builds the StopEvent, and publishes exactly one verdict.
// Pane capture/recovery and verdict disposition stay with the wait coordinator
// so this function has no hidden polling or loop-control side effects.
func (w replWaiter) reviewCheckpoint(state *replWaitState, elapsed int, curPane string, renderWedged bool) checkpointReviewResult {
	state.result.recordTokens(curPane)
	// CB.6: evidence survives the session's death. When the server is killed
	// mid-phase every later capture is empty, so retain the last non-empty pane
	// as the freshest real evidence.
	if strings.TrimSpace(curPane) != "" {
		state.result.lastGoodPane = curPane
	}
	evidencePane := state.result.lastGoodPane

	// Exhaustion is decided separately on a stripped, persistence-gated pane;
	// it must not replace the converging/idle signal given to the reviewer.
	state.livenessCenter.Observe(w.launch.session, curPane, state.livenessProfile)
	livenessState := state.livenessCenter.Aggregate()
	walled := state.livenessCenter.ExhaustedOf(
		strippedForExhaustionScan(curPane, w.responder.injectedPrompt),
		state.paneProfile,
	)
	gateCrossed := state.checkpointExhaustion.observe(walled)
	escalate, suppressNow := state.checkpointWall.decide(
		w.ctx, w.deps.CorroborateWall, w.launch.name, gateCrossed,
	)
	if escalate {
		fmt.Fprintf(w.deps.Stderr, "%s EXHAUSTED: pane shows a quota/rate-limit wall (persisted %d checkpoints, corroborated by live probe) — failing over to fallback CLI (exit %d)\n", w.prefix, exhaustionPersistObservations, ExitUnknownPrompt)
		return checkpointFailover
	}
	if suppressNow {
		fmt.Fprintf(w.deps.Stderr, "%s EXHAUSTION-SUPPRESSED: pane matched wall vocabulary but a live probe (cheapest tier) answered — treating as content-induced; a tier-scoped wall would surface via artifact-timeout fallback instead\n", w.prefix)
	}

	// Changed compares consecutive checkpoint observations. A blank pane from a
	// live session is a render wedge and therefore busy-stagnant, never idle.
	progressed := state.livenessCenter.Changed(w.launch.session)
	if renderWedged && livenessState == panestream.LivenessIdle {
		livenessState = panestream.LivenessBusyButStagnant
	}
	state.lastEvent = StopEvent{
		Kind:       StopArtifactTimeout,
		Phase:      w.cfg.Agent,
		Cycle:      w.cfg.Cycle,
		ElapsedS:   elapsed,
		IntervalS:  state.intervalS,
		Attempt:    state.attempt,
		Progressed: progressed,
		Busy:       state.livenessCenter.Busy(w.launch.session) || renderWedged,
		StdoutTail: lastLines(evidencePane, 40),
		// The fatal and exhaustion detectors strip against the same delivered
		// prompt, so they cannot disagree about agent-authored prompt echoes.
		InjectedPrompt: w.responder.injectedPrompt,
		State:          livenessState,
	}

	// The persistence gate is called exactly once per checkpoint. At enforce,
	// a crossed fatal gate preempts the reviewer and records one durable C2
	// outcome; every other observation falls through to the reviewer.
	verdict, preempted := state.checkpointFatal.verdict(
		state.fatalDetector,
		state.lastEvent,
		state.recoveryStage,
		w.recorder,
		w.deps.Stderr,
		w.prefix,
	)
	if !preempted {
		verdict = state.reviewer.Review(state.lastEvent)
	}
	state.lastVerdict = verdict
	fmt.Fprintf(w.deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
		w.prefix, StopArtifactTimeout, elapsed, state.attempt, progressed, verdict.Action, verdict.Reason)
	if w.deps.OnStopReview != nil {
		w.deps.OnStopReview(w.phaseName, string(verdict.Action), verdict.Reason)
	}
	return checkpointReviewed
}
