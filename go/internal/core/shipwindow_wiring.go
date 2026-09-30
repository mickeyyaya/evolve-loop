package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

var shipWindowAcquireTimeout = shipwindow.DefaultTTL

func (cr *cycleRun) acquireShipWindow(next Phase) {
	if next != PhaseAudit {
		return
	}
	cr.releaseShipWindow()
	ctx, cancel := context.WithTimeout(cr.ctx, shipWindowAcquireTimeout)
	defer cancel()
	l, err := shipwindow.Acquire(ctx, filepath.Join(cr.req.ProjectRoot, ".evolve"), shipwindow.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-window lease not acquired (%v); proceeding unleased — sibling contention may force a re-audit\n", err)
		return
	}
	cr.shipLease = l
}

func (cr *cycleRun) releaseShipWindow() {
	if cr.shipLease == nil {
		return
	}
	if err := cr.shipLease.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-window lease release: %v\n", err)
	}
	cr.shipLease = nil
}
