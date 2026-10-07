package observer

import (
	"context"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

const originStallSignal = "phaseStallSignal"

type phaseWatch struct {
	req   core.PhaseRequest
	phase string
	stall time.Duration
}

func (a *CoreAdapter) phaseStallSignal(ctx context.Context, w phaseWatch) func(Event) {
	req, phase := w.req, w.phase
	return func(e Event) {
		if e.Type != eventStallNoProgress && e.Type != eventStallNoOutput {
			return
		}
		kind := signalcenter.KindObserverWarning
		if e.Source == sourcePane {
			kind = signalcenter.KindPaneLiveness
		}
		centerOf(a.Signals).Emit(signalcenter.Event{
			Cycle: req.Cycle, RunID: req.RunID, Phase: phase, Module: signalcenter.ModuleLiveness, Origin: originStallSignal,
			Kind: kind, Code: bridge.CodePhaseStalled, Severity: signalcenter.SeverityWarn, Reason: e.Reason,
			Fields: map[string]string{
				"source": e.Source, "session": e.Session, "busy": strconv.FormatBool(e.Busy),
				"stall_s": strconv.FormatFloat(w.stall.Seconds(), 'f', -1, 64),
			},
		})
		if a.UsageEvidence != nil && ctx.Err() == nil && e.Source == sourcePane {
			a.reportStallUsage(ctx, w)
		}
	}
}

func (a *CoreAdapter) reportStallUsage(ctx context.Context, w phaseWatch) {
	snap, live, err := panewatch.ReadLive(w.req.Workspace, w.phase, runlease.PIDAlive)
	if err != nil || !live || snap.CLI == "" {
		return
	}
	_ = usageevidence.Report(centerOf(a.Signals), usageevidence.Record{
		Cycle: w.req.Cycle, RunID: w.req.RunID, Phase: w.phase, Workspace: w.req.Workspace, Origin: originStallSignal,
		Driver: snap.CLI, Trigger: "stall", Evidence: a.UsageEvidence(ctx, snap.CLI, time.Now().Add(-w.stall)),
	})
}

func centerOf(signals func() *signalcenter.Center) *signalcenter.Center {
	if signals == nil {
		return nil
	}
	return signals()
}
