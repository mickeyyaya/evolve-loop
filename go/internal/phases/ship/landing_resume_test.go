//go:build integration

package ship

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestRun_ALandingThatCannotUnwindStopsTheCycleWithItsCommitJournaled(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	outage := &pushOutage{Failures: 1, Stderr: githubPolicyError, OnPush: func() {
		mustWrite(t, filepath.Join(l.WT, "debris.txt"), "a writer ship did not expect\n")
	}}

	_, err := runShip(t, l.Repo, l.options(outage.Runner(), &sleepLog{}))

	se := wantShipErr(t, err, core.CodeGitLandingUnwindDeclined, core.ShipClassIntegrity, "ship cannot unwind the lane to the audited shape")
	stranded := l.Tip(t)
	if stranded == l.Base || se.Debug["commit"] != stranded || !journalHasSHA(l.Repo, stranded) {
		t.Errorf("lane tip %s, Debug %v, journaled=%v: the commit stays on the lane and journaled for the boundary", stranded, se.Debug, journalHasSHA(l.Repo, stranded))
	}
	for _, step := range []string{"1. In the plane, run `git merge --ff-only " + stranded + "`", "2. Run `evolve sync-main`", "3. Run `evolve ship --push-only`"} {
		if !strings.Contains(se.Message, step) {
			t.Errorf("message %q lacks the operator step %q", se.Message, step)
		}
	}
	if headSHA(t, l.Repo) != l.Base || remoteHeadSHA(t, l.Repo) != l.Base {
		t.Errorf("main=%s origin=%s, want both at %s", headSHA(t, l.Repo), remoteHeadSHA(t, l.Repo), l.Base)
	}
}

type landingLane struct{ shiptest.Lane }

func laneReadyToLand(t *testing.T) landingLane {
	t.Helper()
	return landingLane{shiptest.LaneReadyToLand(t)}
}

func (l landingLane) options(runner CmdRunner, slept *sleepLog) Options {
	return Options{
		Class: ClassCycle, CommitMessage: "evolve-cycle 1830", CycleID: landingCycle, RunID: landingRunID, AuditRound: 1,
		ActiveWorktree: l.WT, WorktreeBaseSHA: l.Base, WorkspacePath: l.WS,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.View,
		RequireBuildExplanationHandoff: true, Runner: runner, Sleep: slept.Sleep,
	}
}

func (l landingLane) failEveryPush(t *testing.T, stderr string) (*pushOutage, sleepLog, RunResult, error) {
	t.Helper()
	outage := &pushOutage{Failures: 1 << 20, Stderr: stderr}
	var slept sleepLog
	res, err := runShip(t, l.Repo, l.options(outage.Runner(), &slept))
	if res.ExitCode == ExitOK {
		t.Fatalf("precondition: every push fails, so the first ship cannot land: %v", res.Logs)
	}
	return outage, slept, res, err
}
