//go:build integration

package shiptest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (l landingLane) wantAuditedShape(t *testing.T) {
	t.Helper()
	head := strings.TrimSpace(shiptest.RunGitOut(t, l.WT, "rev-parse", "HEAD"))
	staged := strings.TrimSpace(shiptest.RunGitOut(t, l.WT, "write-tree"))
	if head != l.Base || staged != l.Audited {
		t.Errorf("lane HEAD=%s staged=%s, want the audited shape: HEAD=%s (the audited base) with the audited tree %s pending, so Audit never measures ship's own commit", head, staged, l.Base, l.Audited)
	}
}

func reauditTheLane(t *testing.T, l landingLane) {
	t.Helper()
	report := filepath.Join(l.WS, "audit-report.md")
	shiptest.MustWrite(t, report, fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle %d (re-audit)\n\nVerdict: PASS\n\nA newer audit of the same tree.\n", shiptest.LandingCycle))
	shiptest.SealTestPredicateEvidence(t, acssuite.EvidenceIdentity{Cycle: shiptest.LandingCycle, RunID: shiptest.LandingRunID, Round: 1, TreeSHA: l.Audited}, report)
	row, err := json.Marshal(map[string]any{
		"ts": "2026-10-07T15:20:00Z", "cycle": shiptest.LandingCycle, "run_id": shiptest.LandingRunID, "worktree_tree_sha": l.Audited,
		"role": "auditor", "kind": "agent_subprocess", "model": "sonnet", "exit_code": 0, "duration_s": "30",
		"artifact_path": report, "artifact_sha256": shiptest.MustHashFile(t, report), "challenge_token": "testtoken123",
		"git_head": shiptest.HeadSHA(t, l.Repo), "tree_state_sha": shiptest.TreeStateSHA(t, l.Repo), "worktree_base_sha": l.Base,
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(l.Repo, ".evolve", "ledger.jsonl")
	prior, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	shiptest.MustWrite(t, ledger, string(prior)+string(row)+"\n")
}

func TestRun_ARetryAfterAFailedPushResumesAtThePushWithoutReverifying(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	_, _, _, err := l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.WantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "")
	landingCommit := l.Tip(t)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("the retry after a failed push must land the prepared commit, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome != "landing-resumed" || containsLog(res, "audit verified") {
		t.Errorf("repair_outcome=%q, audit re-verified=%v: the retry resumes at the push and re-runs no gate", res.RepairOutcome, containsLog(res, "audit verified"))
	}
	if got := shiptest.RemoteHeadSHA(t, l.Repo); got != landingCommit || res.CommitSHA != landingCommit || shiptest.HeadSHA(t, l.Repo) != landingCommit {
		t.Errorf("origin=%s result=%s main=%s, want the prepared lane commit %s everywhere", got, res.CommitSHA, shiptest.HeadSHA(t, l.Repo), landingCommit)
	}
	binding, err := statemap.ReadStateMap(filepath.Join(l.WS, "ship-binding.json"))
	if err != nil || binding["commit_sha"] != landingCommit || binding["audit_bound_tree_sha"] != l.Audited {
		t.Errorf("ship-binding.json = %v (err %v), want the landed commit %s bound to the audited tree %s", binding, err, landingCommit, l.Audited)
	}
}

func TestRun_AFailedPushLeavesSharedMainAtItsPreLandingTip(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)

	l.failEveryPush(t, shiptest.GithubServerError)

	if got := shiptest.HeadSHA(t, l.Repo); got != l.Base {
		t.Errorf("shared main = %s after a failed push, want its pre-landing tip %s: a peer must never fork from or carry an unpushed landing", got, l.Base)
	}
	if l.Tip(t) == l.Base {
		t.Errorf("the lane lost ship's commit; the prepared landing lives on the lane until the push lands")
	}
}

func TestRun_AFailedPushJournalsItsCommitSoPushOnlyCompletesIt(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	landingCommit := l.Tip(t)
	shiptest.RunGit(t, l.Repo, "merge", "--ff-only", "-q", shiptest.LandingBranch)

	res, err := runShip(t, l.Repo, ship.Options{PushOnly: true})

	if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != landingCommit {
		t.Fatalf("push-only = exit %d err %v, origin %s: a prepared landing is journaled before its push, so push-only completes it (want origin %s)", res.ExitCode, err, shiptest.RemoteHeadSHA(t, l.Repo), landingCommit)
	}
}

func TestRun_AServerErrorTwiceIsRetriedWithBackoffInTheStep(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	outage := &shiptest.PushOutage{Failures: 2, Stderr: shiptest.GithubServerError}
	var slept shiptest.SleepLog

	res, err := runShip(t, l.Repo, l.options(outage.Runner(), &slept))

	if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Fatalf("the live 2x500 shape must land in the same ship: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if want := (shiptest.SleepLog{2 * time.Second, 4 * time.Second}); !reflect.DeepEqual(slept, want) || outage.Pushes != 3 {
		t.Errorf("slept %v over %d pushes, want %v over 3: a transport error backs off and retries the identical push", slept, outage.Pushes, want)
	}
}

func TestRun_AResumeDeclinesOnADivergedOriginAndUnwindsTheLaneToTheAuditedShape(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.PushDivergentCommit(t, l.Bare)
	diverged := shiptest.RemoteHeadSHA(t, l.Repo)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	se := shiptest.WantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassPrecondition, "")
	if se.Debug["repair_outcome"] != "needs-reaudit" || !containsLog(res, "does not resume: origin/main") {
		t.Errorf("Debug[repair_outcome] = %q, logs %q: the resume declines on the diverged origin before it pushes the stale commit", se.Debug["repair_outcome"], res.Logs)
	}
	l.wantAuditedShape(t)
	if status := l.Intent(t)["status"]; status != "unwound" {
		t.Errorf("intent status %v, want unwound", status)
	}
	if shiptest.RemoteHeadSHA(t, l.Repo) != diverged || shiptest.HeadSHA(t, l.Repo) != l.Base {
		t.Errorf("origin=%s main=%s, want origin untouched at %s and main at its pre-landing tip %s", shiptest.RemoteHeadSHA(t, l.Repo), shiptest.HeadSHA(t, l.Repo), diverged, l.Base)
	}
}

func TestRun_APolicyRejectionIsNeverRetried(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)

	outage, slept, _, err := l.failEveryPush(t, shiptest.GithubPolicyError)

	shiptest.WantShipErr(t, err, "GIT_PUSH_POLICY_REFUSED", core.ShipClassPrecondition, "")
	if outage.Pushes != 1 || len(slept) != 0 {
		t.Errorf("pushes=%d slept=%v, want exactly one push and no backoff: a policy rejection is final", outage.Pushes, slept)
	}
	l.wantAuditedShape(t)
	if shiptest.HeadSHA(t, l.Repo) != l.Base || l.Intent(t)["status"] != "unwound" {
		t.Errorf("main = %s, intent %v: want main at its pre-landing tip %s and the intent unwound", shiptest.HeadSHA(t, l.Repo), l.Intent(t)["status"], l.Base)
	}
}

func TestRun_APushRaceIsRepairedWithoutBackoff(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	outage := &shiptest.PushOutage{Failures: 1, Stderr: shiptest.GithubPushRace}
	var slept shiptest.SleepLog

	res, err := runShip(t, l.Repo, l.options(outage.Runner(), &slept))

	if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Fatalf("a race whose origin is an ancestor re-pushes after a fetch: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if len(slept) != 0 || res.RepairOutcome != "push-retried" {
		t.Errorf("slept=%v outcome=%q, want no backoff and push-retried: only transport errors back off", slept, res.RepairOutcome)
	}
}

func TestRun_ALandingIntentThatCannotBeRecordedUnwindsAndNeverPushes(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.LockTheLandingDir(t)
	outage := &shiptest.PushOutage{}

	_, err := runShip(t, l.Repo, l.options(outage.Runner(), &shiptest.SleepLog{}))

	shiptest.WantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "ship: ship cannot record the landing intent, so it pushed nothing")
	if outage.Pushes != 0 || shiptest.RemoteHeadSHA(t, l.Repo) != l.Base || shiptest.HeadSHA(t, l.Repo) != l.Base {
		t.Errorf("pushes=%d origin=%s main=%s: without a durable intent nothing is pushed and nothing moves", outage.Pushes, shiptest.RemoteHeadSHA(t, l.Repo), shiptest.HeadSHA(t, l.Repo))
	}
	l.wantAuditedShape(t)
}
