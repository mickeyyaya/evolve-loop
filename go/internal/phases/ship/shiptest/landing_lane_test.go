//go:build integration

package shiptest_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

type landingLane struct{ shiptest.Lane }

func laneReadyToLand(t *testing.T) landingLane {
	t.Helper()
	return landingLane{shiptest.LaneReadyToLand(t)}
}

func (l landingLane) options(runner ship.CmdRunner, slept *shiptest.SleepLog) ship.Options {
	return ship.Options{
		Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1830", CycleID: shiptest.LandingCycle, RunID: shiptest.LandingRunID, AuditRound: 1,
		ActiveWorktree: l.WT, WorktreeBaseSHA: l.Base, WorkspacePath: l.WS,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.View,
		RequireBuildExplanationHandoff: true, Runner: runner, Sleep: slept.Sleep,
	}
}

func (l landingLane) failEveryPush(t *testing.T, stderr string) (*shiptest.PushOutage, shiptest.SleepLog, ship.RunResult, error) {
	t.Helper()
	outage := &shiptest.PushOutage{Failures: 1 << 20, Stderr: stderr}
	var slept shiptest.SleepLog
	res, err := runShip(t, l.Repo, l.options(outage.Runner(), &slept))
	if res.ExitCode == ship.ExitOK {
		t.Fatalf("precondition: every push fails, so the first ship cannot land: %v", res.Logs)
	}
	return outage, slept, res, err
}
