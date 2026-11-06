package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const laneWallDeferredReason = "lane-wall-deferred"

func cycleRunQuotaExit(result core.CycleResult, projectRoot, evolveDir string, fleetLane bool, stdout, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) int {
	var lr loopResult
	cycle := result.Cycle
	result, clauses := pausedFromCheckpoint(result, evolveDir)
	wall := core.Wall{Ends: result.QuotaWall}
	if !fleetLane {
		lr.emitQuotaPause(loopConfig{EvolveDir: evolveDir}, result, stdout, stderr)
		wallPause{result: result, projectRoot: projectRoot, impact: sequentialImpact(projectRoot, cycle), origin: "cycleRunQuotaExit"}.emit(signals)
		return fleet.ExitDeferred
	}
	held, heldErr := heldClaims(projectRoot, cycle)
	err := releaseDeferredLaneClaims(projectRoot, cycle, laneWallDeferredReason, stderr, lifecycle, signals)
	wallPause{result: result, projectRoot: projectRoot, impact: fleetImpact(projectRoot, cycle, laneRelease{held: held, heldErr: heldErr, releaseErr: err}), origin: "cycleRunQuotaExit"}.emit(signals)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not release cycle %d claims: %v\n", cycle, err)
	} else {
		fmt.Fprintf(stderr, "evolve cycle run: fleet lane deferred at a wall (%s): cycle %d claims are pending again (%s); the next lane of the todo resumes the cycle at its paused phase\n", clauses, cycle, laneWallDeferredReason)
	}
	lr.StopReason = wall.StopReason()
	lr.emit(stdout)
	return fleet.ExitDeferred
}

func pausedFromCheckpoint(result core.CycleResult, evolveDir string) (core.CycleResult, string) {
	if len(result.QuotaWall) > 0 {
		return result, core.Wall{Ends: result.QuotaWall}.Clauses()
	}
	qp, _ := detectQuotaPause(evolveDir)
	result.QuotaWall, result.WallPhase = qp.Wall.Ends, qp.Phase
	return result, qp.clauses()
}

func heldClaims(projectRoot string, cycle int) (int, error) {
	survey, err := inboxmover.SurveyClaims(inboxmover.Options{ProjectRoot: projectRoot})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range survey.Claims {
		if c.Holder.Cycle == cycle {
			n++
		}
	}
	return n, nil
}

func unknownClaims(err error) claimImpact {
	return claimImpact("claim state unknown: " + err.Error())
}

type claimImpact string

type laneRelease struct {
	held       int
	heldErr    error
	releaseErr error
}

func fleetImpact(projectRoot string, cycle int, rel laneRelease) claimImpact {
	after, afterErr := heldClaims(projectRoot, cycle)
	switch {
	case rel.releaseErr != nil:
		return claimImpact("claim release failed: " + rel.releaseErr.Error())
	case afterErr != nil:
		return unknownClaims(afterErr)
	case after > 0:
		return claimImpact(fmt.Sprintf("claim release failed: %d still held", after))
	case rel.heldErr != nil:
		return unknownClaims(rel.heldErr)
	case rel.held > 0:
		return "claims released"
	}
	return "no claim was held"
}

func sequentialImpact(projectRoot string, cycle int) claimImpact {
	after, err := heldClaims(projectRoot, cycle)
	switch {
	case err != nil:
		return unknownClaims(err)
	case after > 0:
		return "claim kept for resume"
	}
	return "no claim was held"
}

type wallPause struct {
	result      core.CycleResult
	projectRoot string
	impact      claimImpact
	origin      string
}

func (p wallPause) emit(signals *signalcenter.Center) {
	wall := core.Wall{Ends: p.result.QuotaWall}
	if !wall.HasLogin() {
		return
	}
	impact := string(p.impact)
	if p.result.WallWriteErr != "" {
		impact += "; the checkpoint write failed: " + p.result.WallWriteErr
	}
	signals.Emit(core.WallReport{Cycle: p.result.Cycle, Phase: p.result.WallPhase, Wall: wall, Workspace: cycleWorkspace(p.projectRoot, p.result.Cycle)}.PauseEvent(impact, p.origin))
}

func fleetLaneFromEnv() bool {
	return envchain.BoolValue(os.Getenv(ipcenv.FleetKey), false)
}
