package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	laneMainRedParkedReason = "lane-main-red-parked"
	mainRedParkedStopReason = "main-red-parked"
)

func (lr *loopResult) emitMainRedPark(origin string, cycle int, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: cycle %d is parked on a red main, and its lane is not blamed; the cycle keeps its claim; resume it when main is green: evolve loop --resume\n", origin, cycle)
	return lr.endMainRedPark(stdout)
}

func (lr *loopResult) endMainRedPark(stdout io.Writer) int {
	lr.StopReason = mainRedParkedStopReason
	lr.emit(stdout)
	return fleet.ExitDeferred
}

func cycleRunMainRedExit(cycle int, projectRoot string, fleetLane bool, stdout, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) int {
	var lr loopResult
	if !fleetLane {
		return lr.emitMainRedPark("evolve cycle run", cycle, stdout, stderr)
	}
	if err := releaseDeferredLaneClaims(projectRoot, cycle, laneMainRedParkedReason, stderr, lifecycle, signals); err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not release cycle %d claims: %v\n", cycle, err)
	} else {
		fmt.Fprintf(stderr, "evolve cycle run: fleet lane parked on a red main: cycle %d claims are pending again (%s); the pool resumes the cycle at ship when main is green\n", cycle, laneMainRedParkedReason)
	}
	return lr.endMainRedPark(stdout)
}
