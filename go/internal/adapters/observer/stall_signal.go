package observer

import (
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const originStallSignal = "phaseStallSignal"

func phaseStallSignal(signals func() *signalcenter.Center, req core.PhaseRequest, phase string, stall time.Duration) func(Event) {
	return func(e Event) {
		if e.Type != eventStallNoProgress && e.Type != eventStallNoOutput {
			return
		}
		kind := signalcenter.KindObserverWarning
		if e.Source == sourcePane {
			kind = signalcenter.KindPaneLiveness
		}
		centerOf(signals).Emit(signalcenter.Event{
			Cycle: req.Cycle, RunID: req.RunID, Phase: phase, Module: signalcenter.ModuleLiveness, Origin: originStallSignal,
			Kind: kind, Code: bridge.CodePhaseStalled, Severity: signalcenter.SeverityWarn, Reason: e.Reason,
			Fields: map[string]string{
				"source": e.Source, "session": e.Session, "busy": strconv.FormatBool(e.Busy),
				"stall_s": strconv.FormatFloat(stall.Seconds(), 'f', -1, 64),
			},
		})
	}
}

func centerOf(signals func() *signalcenter.Center) *signalcenter.Center {
	if signals == nil {
		return nil
	}
	return signals()
}
