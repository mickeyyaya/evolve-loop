package bridge

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

type checkpointReviewResult uint8

const (
	checkpointReviewed checkpointReviewResult = iota
	checkpointFailover
)

func (w replWaiter) reviewCheckpoint(state *replWaitState, elapsed int, curPane string, renderWedged bool) checkpointReviewResult {
	state.result.recordTokens(curPane)
	if strings.TrimSpace(curPane) != "" {
		state.result.lastGoodPane = curPane
	}
	evidencePane := state.result.lastGoodPane

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

	progressed := state.livenessCenter.Changed(w.launch.session)
	if renderWedged && livenessState == panestream.LivenessIdle {
		livenessState = panestream.LivenessBusyButStagnant
	}
	state.lastEvent = StopEvent{
		Kind:           StopArtifactTimeout,
		Phase:          w.cfg.Agent,
		Cycle:          w.cfg.Cycle,
		ElapsedS:       elapsed,
		IntervalS:      state.intervalS,
		Attempt:        state.attempt,
		Progressed:     progressed,
		Busy:           state.livenessCenter.Busy(w.launch.session) || renderWedged,
		StdoutTail:     lastLines(evidencePane, 40),
		InjectedPrompt: w.responder.injectedPrompt,
		State:          livenessState,
	}

	verdict, preempted := state.checkpointFatal.verdict(
		state.fatalDetector,
		state.lastEvent,
		state.fatalPaneStage,
		w.recorder,
		w.deps.Stderr,
		w.prefix,
	)
	if preempted {
		observeFatalPane(w.deps, fatalPaneObservation{cause: verdict.Cause, named: w.launch.named, intervalS: state.intervalS})
	} else {
		verdict = state.reviewer.Review(state.lastEvent)
	}
	state.lastVerdict = verdict
	fmt.Fprintf(w.deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
		w.prefix, StopArtifactTimeout, elapsed, state.attempt, progressed, verdict.Action,
		boundedDiagnosticQuote(verdict.Reason, 220))
	if w.deps.OnStopReview != nil {
		w.deps.OnStopReview(w.phaseName, string(verdict.Action), verdict.Reason)
	}
	return checkpointReviewed
}
