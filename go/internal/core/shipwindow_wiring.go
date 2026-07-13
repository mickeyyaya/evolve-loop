package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

// shipWindowAcquireTimeout bounds how long a lane waits for a sibling's
// audit→ship window before proceeding UNLEASED. Fail-open by design: the
// pre-lease behavior (risking an AUDIT_BINDING_HEAD_MOVED re-audit) is
// strictly better than wedging the loop. Var, not const: test seam.
var shipWindowAcquireTimeout = shipwindow.DefaultTTL

// acquireShipWindow serializes the audit→ship critical section across lanes
// (cycle-778 ship-window-lease): taken immediately BEFORE the audit-binding
// HEAD snapshot (emitPhaseBindings → recordAuditBinding) and held through
// ship's push, so a sibling landing on main inside that window queues instead
// of forcing this lane into a deep-tier re-audit. Deliberately NOT held
// across the audit agent itself — only binding-snapshot→push. A re-audit
// while already holding releases first (FIFO fairness to waiting siblings +
// a fresh heartbeat on the new hold). Acquisition failure/timeout WARNs and
// proceeds unleased.
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

// releaseShipWindow frees the ship-window lease if held. Idempotent; called
// when any post-audit phase completes (ship's push is done — or the cycle
// routed away from ship, where holding longer buys nothing) and from
// RunCycle's exit defer so no abort path leaves siblings waiting out the TTL.
func (cr *cycleRun) releaseShipWindow() {
	if cr.shipLease == nil {
		return
	}
	if err := cr.shipLease.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-window lease release: %v\n", err)
	}
	cr.shipLease = nil
}
