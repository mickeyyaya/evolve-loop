package core

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeShipWindowLeaseBroken signalcenter.Code = "ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeShipWindowLeaseBroken, "the ship-window lease was broken. A ship recovery broke the lease of a live holder whose heartbeat passed the TTL (fields.owner_pid and fields.heartbeat_at name the broken holder). Or the holder of a recovery lost its own lease at a beat (fields.role is holder, and fields.reason names the cause). Two lanes can overlap in the window")
}

type shipWindowHold struct {
	mu      sync.Mutex
	lease   *shipwindow.Lease
	options shipwindow.Options
}

var defaultShipWindowTicker shipwindow.Ticker

func WithShipWindowOptions(w shipwindow.Options) Option {
	return func(o *Orchestrator) { o.shipWindow.options = w }
}

func (o *Orchestrator) holdShipWindow(ctx context.Context, projectRoot string, cycle int, runID string) {
	o.shipWindow.mu.Lock()
	defer o.shipWindow.mu.Unlock()
	if o.shipWindow.lease != nil {
		return
	}
	lease, err := shipwindow.Acquire(ctx, filepath.Join(projectRoot, ".evolve"), o.shipWindowOptions(cycle, runID))
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN ship-window lease not acquired (%v); the recovery proceeds unleased\n", err)
		return
	}
	o.shipWindow.lease = lease
}

func (o *Orchestrator) shipWindowOptions(cycle int, runID string) shipwindow.Options {
	w := o.shipWindow.options
	if w.Ticker == nil {
		w.Ticker = defaultShipWindowTicker
	}
	w.OnStaleBreak = func(h shipwindow.StaleHolder) { o.reportShipWindowBreak(cycle, runID, h) }
	w.OnLost = func(e *shipwindow.LostError) { o.reportShipWindowLost(cycle, runID, e) }
	return w
}

func (o *Orchestrator) releaseShipWindow() {
	o.shipWindow.mu.Lock()
	defer o.shipWindow.mu.Unlock()
	if o.shipWindow.lease == nil {
		return
	}
	if err := o.shipWindow.lease.Release(); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN ship-window lease release: %v\n", err)
	}
	o.shipWindow.lease = nil
}

func (o *Orchestrator) shipWindowContext(ctx context.Context) context.Context {
	o.shipWindow.mu.Lock()
	defer o.shipWindow.mu.Unlock()
	if o.shipWindow.lease == nil {
		return ctx
	}
	return shipwindow.WithHeld(ctx, o.shipWindow.lease)
}

func (o *Orchestrator) keepShipWindowOnlyForShip(next Phase, ok bool) (Phase, bool) {
	if !ok || next != PhaseShip {
		o.releaseShipWindow()
	}
	return next, ok
}

func (o *Orchestrator) runPhase(ctx context.Context, phase Phase, runner PhaseRunner, req PhaseRequest) (PhaseResponse, error) {
	if phase != PhaseShip {
		return runner.Run(ctx, req)
	}
	defer o.releaseShipWindow()
	return runner.Run(o.shipWindowContext(ctx), req)
}

func (o *Orchestrator) reportShipWindowLost(cycle int, runID string, e *shipwindow.LostError) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: runID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.holdShipWindow",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeShipWindowLeaseBroken,
		Reason: e.Error(), Fields: e.Fields(),
	})
}

func (o *Orchestrator) reportShipWindowBreak(cycle int, runID string, h shipwindow.StaleHolder) {
	reason := h.Reason()
	fmt.Fprintf(o.errOut(), "[orchestrator] WARN %s\n", reason)
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: runID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.holdShipWindow",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeShipWindowLeaseBroken,
		Reason: reason, Fields: h.Fields(),
	})
}
