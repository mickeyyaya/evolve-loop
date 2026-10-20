package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const laneQuotaDeferredReason = "lane-quota-deferred"

func cycleRunQuotaExit(cycle int, projectRoot, evolveDir string, fleetLane bool, stdout, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) int {
	var lr loopResult
	lr.emitQuotaPause(loopConfig{EvolveDir: evolveDir}, cycle, stdout, stderr)
	if fleetLane {
		if err := releaseDeferredLaneClaims(projectRoot, cycle, laneQuotaDeferredReason, stderr, lifecycle, signals); err != nil {
			fmt.Fprintf(stderr, "evolve cycle run: WARN: could not release cycle %d claims: %v\n", cycle, err)
		} else {
			fmt.Fprintf(stderr, "evolve cycle run: fleet lane deferred on a quota wall: cycle %d claims are pending again (%s); the next wave plans them, no resume needed\n", cycle, laneQuotaDeferredReason)
		}
	}
	return fleet.ExitDeferred
}
