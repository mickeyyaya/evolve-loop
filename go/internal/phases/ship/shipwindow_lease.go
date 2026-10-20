package ship

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const codeShipWindowLeaseBroken signalcenter.Code = "SHIP_WINDOW_LEASE_BROKEN"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, codeShipWindowLeaseBroken, "a ship broke the ship-window lease of a live holder whose heartbeat passed the TTL, so two lanes can overlap in the window; fields.owner_pid and fields.heartbeat_at name the broken holder")
}

func enterShipWindow(ctx context.Context, opts *Options, res *RunResult) func() {
	if opts.Class != ClassCycle || opts.DryRun || shipwindow.Held(ctx) {
		return func() {}
	}
	lease, err := shipwindow.Acquire(ctx, filepath.Join(opts.ProjectRoot, ".evolve"), shipwindow.Options{
		OnStaleBreak: func(h shipwindow.StaleHolder) { reportShipWindowBreak(opts, res, h) },
	})
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: ship-window lease not acquired (%v); proceeding unleased", err))
		return func() {}
	}
	return func() {
		if err := lease.Release(); err != nil {
			res.Logs = append(res.Logs, "[ship] WARN: ship-window lease release: "+err.Error())
		}
	}
}

func reportShipWindowBreak(opts *Options, res *RunResult, h shipwindow.StaleHolder) {
	reason := h.Reason()
	res.Logs = append(res.Logs, "[ship] WARN: "+reason)
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "enterShipWindow",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: codeShipWindowLeaseBroken,
		Reason: reason, Fields: h.Fields(),
	})
}
