package bridge

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// replWaitStep tells the coordinator whether an extracted wait transition
// continues polling or ends the loop with an exit code.
type replWaitStep struct {
	done bool
	code int
}

// handleTickInteractions applies operator inbox input before auto-response for
// one incomplete completion tick. Pane capture, streaming, and completion
// polling stay in the coordinator so their ordering remains explicit.
func (w replWaiter) handleTickInteractions(state *replWaitState, elapsed int, pane string, captureOK bool) replWaitStep {
	// Drain live-injection envelopes BEFORE the auto-respond tick so an
	// operator interrupt pre-empts a pending auto-reply on this tick.
	if envs, _ := w.cursor.Drain(); len(envs) > 0 {
		for _, env := range envs {
			// injectEnvelope returns a non-empty CorrID only when an
			// idle-gated correlated ask was actually pasted (not re-queued,
			// dropped, or a keystroke/interrupt). The breadcrumb is emitted
			// HERE — at the moment delivery is confirmed — so the channel sink
			// and open-span tracking share the same owner. Channel-off delivery
			// remains inert inside injectionDelivered.
			cid := injectEnvelope(w.ctx, w.cfg, w.deps, w.launch, env)
			w.channel.injectionDelivered(cid)
		}
	}

	w.channel.observeIdle(pane, state.livenessCenter)
	action, rc := w.responder.tickPane(w.ctx, w.launch.session, pane, captureOK)
	switch rc {
	case 0, 1: // noop / responded
	case 2:
		// Agent self-signalled progress ("extend_timeout"): restart the
		// current review interval so the signal counts as activity. Bounded
		// by the auto-respond loop guard (case 86) — an agent cannot defer
		// the reviewer indefinitely by repeating the same extend prompt.
		if parseExtendSecs(action) > 0 {
			state.intervalStartS = elapsed
			fmt.Fprintf(w.deps.Stderr, "%s agent extend signal — review interval refreshed\n", w.prefix)
		}
	case 3:
		if strings.TrimSpace(pane) != "" {
			state.result.lastGoodPane = pane
		}
		state.lastEvent = StopEvent{
			Kind:           StopArtifactTimeout,
			Phase:          w.cfg.Agent,
			Cycle:          w.cfg.Cycle,
			ElapsedS:       elapsed,
			IntervalS:      state.intervalS,
			Busy:           false,
			StdoutTail:     lastLines(state.result.lastGoodPane, 40),
			InjectedPrompt: w.responder.injectedPrompt,
			State:          panestream.LivenessIdle,
		}
		state.lastVerdict = ReviewVerdict{Action: ReviewStop, Reason: "transient upstream error persisted for 60s"}
		state.transientShortcircuit = true
		return replWaitStep{done: true}
	case 85:
		fmt.Fprintf(w.deps.Stderr, "%s auto-respond escalation; abandoning run\n", w.prefix)
		return replWaitStep{done: true, code: ExitUnknownPrompt}
	case 86:
		fmt.Fprintf(w.deps.Stderr, "%s auto-respond loop guard tripped; abandoning run\n", w.prefix)
		return replWaitStep{done: true, code: ExitRespondLoopGuard}
	}
	return replWaitStep{}
}
