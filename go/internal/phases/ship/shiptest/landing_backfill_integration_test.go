//go:build integration

package shiptest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestRun_APolicyRefusalWhoseUnwoundIntentCannotBeWrittenReportsBoth(t *testing.T) {
	t.Parallel()
	shiptest.SkipWhenRoot(t)
	l := laneReadyToLand(t)
	outage := &shiptest.PushOutage{Failures: 1 << 20, Stderr: shiptest.GithubPolicyError, OnPush: func() { l.LockTheLandingDir(t) }}

	_, err := runShip(t, l.Repo, l.options(outage.Runner(), &shiptest.SleepLog{}))

	shiptest.WantShipErr(t, err, "GIT_PUSH_POLICY_REFUSED", core.ShipClassPrecondition, "")
	if err == nil || !strings.Contains(err.Error(), "ship cannot mark the landing intent unwound") {
		t.Errorf("err %v, want the policy refusal joined with the failed intent write", err)
	}
	l.wantAuditedShape(t)
	if status := l.Intent(t)["status"]; status != "prepared" || outage.Pushes != 1 {
		t.Errorf("intent %v, pushes %d: the unwind ran once and the intent stays prepared on disk", status, outage.Pushes)
	}
}

func TestRun_APostPushReDispatchWithATamperedAuditFailsInsteadOfReportingSuccess(t *testing.T) {
	t.Parallel()
	repo, _ := shiptest.ShipWorktreeFixture(t, "cycle-1-tamper")
	if res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"}); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the first ship lands: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	landed := shiptest.RemoteHeadSHA(t, repo)
	report := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	shiptest.MustWrite(t, report, string(body)+"\nan edit after the audit\n")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})

	shiptest.WantShipErr(t, err, core.CodeAuditBindingArtifactSHA, core.ShipClassPrecondition, "artifact mutated post-audit")
	if res.ExitCode != ship.ExitFailure || res.CommitSHA != "" || containsLog(res, "succeeding report-only") || shiptest.RemoteHeadSHA(t, repo) != landed {
		t.Errorf("exit=%d commit=%q logs=%q: a post-push re-dispatch whose audit changed reports a failure, never a success", res.ExitCode, res.CommitSHA, res.Logs)
	}
}

func TestRun_AResumeUnpushedWhosePushIsRefusedLeavesTheHeadMovedError(t *testing.T) {
	t.Parallel()
	repo := shiptest.MergedUnpushedFixture(t, "cycle-246-refused")
	preRemote := shiptest.RemoteHeadSHA(t, repo)
	outage := &shiptest.PushOutage{Failures: 1 << 20, Stderr: shiptest.GithubPolicyError}

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test", Runner: outage.Runner()})

	if se := shiptest.MustShipErr(t, err); se.Code != core.CodeAuditBindingHeadMoved {
		t.Errorf("code = %s, want the head-moved refusal to stand", se.Code)
	}
	if res.ExitCode == ship.ExitOK || res.RepairOutcome == "resume-pushed" || outage.Pushes == 0 || shiptest.RemoteHeadSHA(t, repo) != preRemote {
		t.Errorf("exit=%d outcome=%q pushes=%d origin=%s: a refused push never completes the resume", res.ExitCode, res.RepairOutcome, outage.Pushes, shiptest.RemoteHeadSHA(t, repo))
	}
}
