package bridge

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

type replWaitStep struct {
	done bool
	code int
}

func (w replWaiter) handleTickInteractions(state *replWaitState, elapsed int, pane string, captureOK bool) replWaitStep {
	if envs, _ := w.cursor.Drain(); len(envs) > 0 {
		for _, env := range envs {
			cid := injectEnvelope(w.ctx, w.cfg, w.deps, w.launch, env)
			w.channel.injectionDelivered(cid)
		}
	}

	w.observeTickPane(state, pane, captureOK)
	action, rc := w.responder.tickPane(w.ctx, w.launch.session, pane, captureOK)
	switch rc {
	case 0, 1: // noop / responded
	case 2:
		// Agent self-signalled progress restarts the interval so the extend
		// counts as activity.
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
