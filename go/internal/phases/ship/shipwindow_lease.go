package ship

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeShipWindowLeaseBroken signalcenter.Code = "SHIP_WINDOW_LEASE_BROKEN"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeShipWindowLeaseBroken, "the ship-window lease was broken. A ship broke the lease of a live holder whose heartbeat passed the TTL (fields.owner_pid and fields.heartbeat_at name the holder). Or the holder lost its own lease at a beat (fields.role is holder, and fields.reason names the cause). Two lanes can overlap in the window")
}

func enterShipWindow(ctx context.Context, opts *Options, res *RunResult) (context.Context, func()) {
	if opts.Class != ClassCycle || opts.DryRun || shipwindow.Held(ctx) {
		return ctx, func() {}
	}
	lease, err := shipwindow.Acquire(ctx, filepath.Join(opts.ProjectRoot, ".evolve"), opts.shipWindowOptions(res))
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: ship-window lease not acquired (%v); proceeding unleased", err))
		return ctx, func() {}
	}
	return shipwindow.WithHeld(ctx, lease), func() {
		if err := lease.Release(); err != nil {
			res.Logs = append(res.Logs, "[ship] WARN: ship-window lease release: "+err.Error())
		}
	}
}

func (o *Options) shipWindowOptions(res *RunResult) shipwindow.Options {
	w := o.ShipWindow
	w.OnStaleBreak = func(h shipwindow.StaleHolder) { reportShipWindowBreak(o, res, h) }
	w.OnLost = func(e *shipwindow.LostError) { reportShipWindowLost(o, e) }
	return w
}

func requireShipWindow(ctx context.Context, res *RunResult) error {
	lost := shipwindow.HeldErr(ctx)
	if lost == nil {
		return nil
	}
	res.Logs = append(res.Logs, "[ship] ABORT: "+lost.Error()+"; ship pushed nothing")
	return shipErr(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip,
		"ship: the ship-window lease was lost before the push ("+lost.Error()+"); a peer may have moved main, so rebase and re-verify, then re-ship")
}

func reportShipWindowBreak(opts *Options, res *RunResult, h shipwindow.StaleHolder) {
	reason := h.Reason()
	res.Logs = append(res.Logs, "[ship] WARN: "+reason)
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "enterShipWindow",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeShipWindowLeaseBroken,
		Reason: reason, Fields: h.Fields(),
	})
}

func reportShipWindowLost(opts *Options, e *shipwindow.LostError) {
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "enterShipWindow",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeShipWindowLeaseBroken,
		Reason: e.Error(), Fields: e.Fields(),
	})
}
