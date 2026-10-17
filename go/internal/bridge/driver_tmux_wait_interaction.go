package bridge

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeInboxDrainFailed signalcenter.Code = "BRIDGE_INBOX_DRAIN_FAILED"

	inboxDrainOrigin = "replWaiter.handleTickInteractions"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeInboxDrainFailed, "the inbox of the running dispatch could not be read at a tick; the envelopes after the read offset stay unread and the next tick tries again; fields dispatch, agent, error")
}

type replWaitStep struct {
	done bool
	code int
}

func (w replWaiter) reportInboxDrainFailure(err error) {
	fmt.Fprintf(w.deps.Stderr, "%s WARN: inbox drain failed: %v (unread envelopes stay queued for the next tick)\n", w.prefix, err)
	w.deps.Signals.Emit(signalcenter.Event{Module: signalcenter.ModuleBridge, Origin: inboxDrainOrigin, Kind: signalcenter.KindBridgeWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeInboxDrainFailed, Reason: "the inbox of the running dispatch could not be read",
		Fields: map[string]string{"dispatch": w.cfg.DispatchID, "agent": w.cfg.Agent, "error": err.Error()}})
}

func (w replWaiter) handleTickInteractions(state *replWaitState, elapsed int, pane string, captureOK bool) replWaitStep {
	envs, drainErr := w.cursor.Drain()
	if drainErr != nil && !state.inboxDrainWarned {
		state.inboxDrainWarned = true
		w.reportInboxDrainFailure(drainErr)
	}
	for _, env := range envs {
		cid := injectEnvelope(w.ctx, w.cfg, w.deps, w.launch, env)
		w.channel.injectionDelivered(cid)
	}

	w.observeTickPane(state, pane, captureOK)
	action, rc := w.responder.tickPane(w.ctx, w.launch.session, pane, captureOK)
	switch rc {
	case 0, 1:
	case 2:
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
