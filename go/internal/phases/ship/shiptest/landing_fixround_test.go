//go:build integration

package shiptest_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (l landingLane) rewindToPrepared(t *testing.T) {
	t.Helper()
	l.setIntentField(t, "status", "prepared")
	if err := os.Remove(filepath.Join(l.WS, "ship-binding.json")); err != nil {
		t.Fatal(err)
	}
}

func (l *landingLane) pendTheAuditedChangeAgain(t *testing.T) {
	t.Helper()
	shiptest.RunGit(t, l.WT, "reset", "-q", "--hard", l.Base)
	shiptest.RunGit(t, l.WT, "read-tree", "-m", "-u", l.Base, l.Audited)
}

func (l *landingLane) rebuildWithAnotherChange(t *testing.T) {
	t.Helper()
	l.pendTheAuditedChangeAgain(t)
	shiptest.MustWrite(t, filepath.Join(l.WT, "fixture.txt"), "fixture line 1\nthe audited cycle work, revised by a repair round\n")
	binding := explanationdocs.CycleBinding{ProjectRoot: l.Repo, Worktree: l.WT, Workspace: l.WS, BaseSHA: l.Base,
		Cycle: shiptest.LandingCycle, RunID: shiptest.LandingRunID, ContractVersion: explanationdocs.CurrentContractVersion}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	view, err := explanationdocs.Load(l.WS)
	if err != nil {
		t.Fatal(err)
	}
	l.View = view
	shiptest.RunGit(t, l.WT, "add", "-A")
	l.Audited = strings.TrimSpace(shiptest.RunGitOut(t, l.WT, "write-tree"))
}

func appendAnAuditOfAnotherRun(t *testing.T, l landingLane) {
	t.Helper()
	report := filepath.Join(t.TempDir(), "audit-report.md")
	shiptest.MustWrite(t, report, "# Audit Report — another run\n\nVerdict: PASS\n")
	row, err := json.Marshal(map[string]any{
		"ts": "2026-10-07T15:30:00Z", "cycle": shiptest.LandingCycle, "run_id": "run-1830-other", "worktree_tree_sha": l.Base,
		"role": "auditor", "kind": "agent_subprocess", "model": "sonnet", "exit_code": 0, "duration_s": "30",
		"artifact_path": report, "artifact_sha256": shiptest.MustHashFile(t, report), "git_head": l.Base, "tree_state_sha": "x",
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

func failFastForwards(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if name == "git" && len(args) > 1 && args[0] == "merge" && args[1] == "--ff-only" {
		_, _ = io.WriteString(stderr, "fatal: Unable to create '.git/index.lock': File exists.\n")
		return 128, nil
	}
	return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
}

func hasRef(repo, ref string) bool {
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", ref)
	cmd.Env = shiptest.FilteredEnv()
	return cmd.Run() == nil
}

func TestRun_ASupersedingAuditOfAnotherTreeMarksTheIntentStaleAndTheGatesBindIt(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	stale := l.Tip(t)
	l.rebuildWithAnotherChange(t)
	reauditTheLane(t, l)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("a newer audit of another tree re-ships through the gates, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	binding, berr := statemap.ReadStateMap(filepath.Join(l.WS, "ship-binding.json"))
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "audit verified") || berr != nil ||
		binding["commit_sha"] == stale || binding["commit_sha"] != shiptest.RemoteHeadSHA(t, l.Repo) ||
		binding["audit_bound_tree_sha"] != l.Audited {
		t.Errorf("outcome=%q binding=%v: the newer audit binds the landing, never the stale commit %s", res.RepairOutcome, binding, stale)
	}
}

func TestRun_ASupersededIntentOnItsOwnCommitUnwindsBeforeTheGates(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	reauditTheLane(t, l)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("a superseded intent on its own commit unwinds and re-ships, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "does not resume: a newer audit") ||
		!containsLog(res, "ship unwound the lane") || !containsLog(res, "audit verified") || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Errorf("outcome=%q logs=%q: the lane unwinds to the audited shape, the gates bind the newer audit, and the landing completes", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AResumeSettlesACommitOriginHoldsUnderAPeerLanding(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{})); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.Tip(t)
	l.rewindToPrepared(t)
	peer := shiptest.PushOnTopOf(t, l.Bare, landed)
	shiptest.RunGit(t, l.Repo, "fetch", "-q", "origin", "main")
	shiptest.RunGit(t, l.Repo, "merge", "--ff-only", "-q", "origin/main")

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("origin already holds %s under the peer %s, so the resume settles it: exit=%d err=%v logs=%v", landed, peer, res.ExitCode, err, res.Logs)
	}
	binding, _ := statemap.ReadStateMap(filepath.Join(l.WS, "ship-binding.json"))
	if binding["commit_sha"] != landed || shiptest.HeadSHA(t, l.Repo) != peer || shiptest.RemoteHeadSHA(t, l.Repo) != peer || l.Intent(t)["status"] != "complete" {
		t.Errorf("binding=%v main=%s origin=%s intent=%v: no push, no unwind, the binding names %s and the intent completes", binding, shiptest.HeadSHA(t, l.Repo), shiptest.RemoteHeadSHA(t, l.Repo), l.Intent(t)["status"], landed)
	}
}

func TestRun_AResumeFastForwardsMainToACommitAPeerPushedOnTopOf(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{})); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.Tip(t)
	l.rewindToPrepared(t)
	shiptest.RunGit(t, l.Repo, "reset", "-q", "--hard", l.Base)
	peer := shiptest.PushOnTopOf(t, l.Bare, landed)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("a kill after the push left main at %s; origin holds %s under the peer %s: exit=%d err=%v logs=%v", l.Base, landed, peer, res.ExitCode, err, res.Logs)
	}
	if shiptest.HeadSHA(t, l.Repo) != landed || shiptest.RemoteHeadSHA(t, l.Repo) != peer || l.Intent(t)["status"] != "complete" {
		t.Errorf("main=%s origin=%s intent=%v: main fast-forwards to the landed %s, origin stays at the peer", shiptest.HeadSHA(t, l.Repo), shiptest.RemoteHeadSHA(t, l.Repo), l.Intent(t)["status"], landed)
	}
}

func TestRun_ALaggingMainCatchesUpBeforeTheNextLandingChecksItsFastForward(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.Repo, l.options(failFastForwards, &shiptest.SleepLog{})); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the first landing pushes and only its fast-forward fails: %v %v", err, res.Logs)
	}
	landed := shiptest.RemoteHeadSHA(t, l.Repo)
	next := shiptest.MakeWorktree(t, l.Repo, "cycle-1831-branch")
	shiptest.MustWrite(t, filepath.Join(next, "next.txt"), "the next lane's work\n")
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	shiptest.SeedAudit(t, l.Repo, "PASS")
	outage := &shiptest.PushOutage{}

	_, err := runShip(t, l.Repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1831", Runner: outage.Runner(), Env: map[string]string{"EVOLVE_FLEET": "1"}})

	shiptest.WantShipErr(t, err, core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	if shiptest.HeadSHA(t, l.Repo) != landed || outage.Pushes != 0 || shiptest.RemoteHeadSHA(t, l.Repo) != landed {
		t.Errorf("main=%s pushes=%d origin=%s: main catches up to the journaled %s first, so the next lane's divergence is a fleet rebase, not a push race", shiptest.HeadSHA(t, l.Repo), outage.Pushes, shiptest.RemoteHeadSHA(t, l.Repo), landed)
	}
}

func (l landingLane) setIntentField(t *testing.T, key string, value any) {
	t.Helper()
	in := l.Intent(t)
	in[key] = value
	l.WriteIntentMap(t, in)
}

func TestRun_AResumeTakesTheBranchFromTheHostNeverFromTheIntent(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	l.setIntentField(t, "branch", "forged")

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) || hasRef(l.Bare, "refs/heads/forged") {
		t.Errorf("outcome=%q err=%v origin=%s forged-ref=%v: the resume pushes the host's branch main, whatever branch the intent names", res.RepairOutcome, err, shiptest.RemoteHeadSHA(t, l.Repo), hasRef(l.Bare, "refs/heads/forged"))
	}
}

func TestRun_APreparedIntentOfAnotherRunNeverStopsAPendingLane(t *testing.T) {
	t.Parallel()
	for name, options := range map[string]func(landingLane) ship.Options{
		"the phase path names the run": func(l landingLane) ship.Options { return l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}) },
		"the CLI path names no run": func(landingLane) ship.Options {
			return ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1830", Runner: sysexec.DefaultRunner}
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := laneReadyToLand(t)
			l.WriteIntentMap(t, map[string]any{"cycle": shiptest.LandingCycle, "run_id": "run-older", "audit_artifact_sha256": strings.Repeat("a", 64),
				"audited_tree": l.Audited, "lane_tree": l.Audited, "worktree_base_sha": l.Base, "commit_sha": l.Base, "commit_tree": l.Audited,
				"pre_main": l.Base, "branch": "main", "status": "prepared"})

			res, err := runShip(t, l.Repo, options(l))

			if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
				t.Errorf("exit=%d err=%v logs=%q: the witness takes the run from the host, so an intent of another run is stale and the lane ships through its gates", res.ExitCode, err, res.Logs)
			}
		})
	}
}

func TestRun_AResumeNeverUnwindsALaneTipShipDidNotPrepare(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.RunGit(t, l.WT, "-c", "commit.gpgsign=false", "commit", "-q", "--amend", "-m", "a lane tip ship did not prepare")
	moved := l.Tip(t)

	res, _ := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if l.Tip(t) != moved || containsLog(res, "ship unwound the lane") || shiptest.RemoteHeadSHA(t, l.Repo) == moved {
		t.Errorf("lane=%s logs=%q: ship never unwinds or pushes a lane tip %s that it did not prepare", l.Tip(t), res.Logs, moved)
	}
	if status := l.Intent(t)["status"]; status != "stale" {
		t.Errorf("intent status %v, want stale", status)
	}
}

func TestRun_AResumeAfterTheOrchestratorPendedTheLaneMarksTheIntentStale(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	l.pendTheAuditedChangeAgain(t)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Fatalf("a lane pended again by the orchestrator ships through its gates, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "does not resume: the lane tip moved") || containsLog(res, "ship unwound the lane") {
		t.Errorf("outcome=%q logs=%q: the moved lane is not unwound; the intent goes stale", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AFailedWriteOfTheUnwoundIntentIsAnError(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.PushDivergentCommit(t, l.Bare)
	l.LockTheLandingDir(t)

	_, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	shiptest.WantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "landing intent unwound")
	l.wantAuditedShape(t)
}

func TestRun_AResumeBindsTheNewestAuditOfItsOwnRun(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	appendAnAuditOfAnotherRun(t, l)

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" {
		t.Errorf("outcome=%q err=%v: a newer audit of another run never supersedes this run's landing", res.RepairOutcome, err)
	}
}

func TestRun_AKillBeforeTheLaneRefMovesRollsTheLaneRefForward(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	killed := false
	kill := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && !killed && slices.Contains(args, "update-ref") {
			killed = true
			return 1, nil
		}
		if name == "git" && len(args) > 0 && args[0] == "push" {
			_, _ = io.WriteString(stderr, shiptest.GithubServerError)
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	if res, _ := runShip(t, l.Repo, l.options(kill, &shiptest.SleepLog{})); res.ExitCode == ship.ExitOK {
		t.Fatalf("precondition: the first ship stops before it lands: %v", res.Logs)
	}

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || !containsLog(res, "moves the lane ref forward") || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Errorf("exit=%d outcome=%q err=%v logs=%q: the intent and the journal exist before the lane ref moves, so the retry moves the ref forward and resumes", res.ExitCode, res.RepairOutcome, err, res.Logs)
	}
}

func TestRun_APeerLaneOnAStrandedLandingStopsWithItsSteps(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.RunGit(t, l.Repo, "merge", "--ff-only", "-q", shiptest.LandingBranch)
	next := shiptest.MakeWorktree(t, l.Repo, "cycle-1831-branch")
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	shiptest.SeedAudit(t, l.Repo, "PASS")

	res, err := runShip(t, l.Repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1831"})

	se := shiptest.WantShipErr(t, err, "GIT_LANE_NOT_ON_ORIGIN", core.ShipClassIntegrity, "")
	for _, step := range []string{"1. In the plane, run `git merge --ff-only " + l.Tip(t) + "`", "2. Run `evolve sync-main`", "3. Run `evolve ship --push-only`"} {
		if !strings.Contains(se.Message, step) {
			t.Errorf("message %q lacks the step %q", se.Message, step)
		}
	}
	if res.ExitCode == ship.ExitOK {
		t.Error("a lane on a stranded landing never passes with no push")
	}
}

func TestRun_ALaneOnAMainAheadByBoundaryCommitsIsStillNothingToShip(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")
	published := shiptest.HeadSHA(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, "docs", "dossier.md"), "a dossier closeout\n")
	shiptest.RunGit(t, repo, "add", "-A")
	shiptest.RunGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "dossier: cycle-1830 closeout")
	wt := shiptest.MakeWorktree(t, repo, "cycle-1831-branch")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1"})

	if err != nil || res.ExitCode != ship.ExitOK || !containsLog(res, "exiting cleanly") || shiptest.RemoteHeadSHA(t, repo) != published {
		t.Errorf("exit=%d err=%v: a boundary commit ahead of origin is no stranded landing, so a lane with nothing to ship exits cleanly", res.ExitCode, err)
	}
}

func TestRun_ALaneOnACompletedLandingOriginLostIsStillNothingToShip(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{})); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the landing completes: %v %v", err, res.Logs)
	}
	shiptest.RunGit(t, l.Bare, "update-ref", "refs/heads/main", l.Base)
	shiptest.RunGit(t, l.Repo, "update-ref", "refs/remotes/origin/main", l.Base)
	next := shiptest.MakeWorktree(t, l.Repo, "cycle-1831-branch")
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	shiptest.SeedAudit(t, l.Repo, "PASS")

	res, err := runShip(t, l.Repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1831"})

	if err != nil || res.ExitCode != ship.ExitOK || !containsLog(res, "exiting cleanly") {
		t.Errorf("exit=%d err=%v: a journaled lane commit whose intent is complete is no stranded landing", res.ExitCode, err)
	}
}

func TestRun_AnUnreadableCycleStateStopsTheResumeLoudly(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	shiptest.MustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), "{")

	_, err := runShip(t, l.Repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1830", Runner: sysexec.DefaultRunner})

	shiptest.WantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "landing intent")
}

func TestRun_AStopAfterThePushAndBeforeTheFastForwardSettlesWithNoPush(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{})); err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.Tip(t)
	l.rewindToPrepared(t)
	shiptest.RunGit(t, l.Repo, "reset", "-q", "--hard", l.Base)
	outage := &shiptest.PushOutage{}

	res, err := runShip(t, l.Repo, l.options(outage.Runner(), &shiptest.SleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || shiptest.HeadSHA(t, l.Repo) != landed || l.Intent(t)["status"] != "complete" {
		t.Errorf("outcome=%q err=%v main=%s: the resume settles the pushed %s and fast-forwards main to it", res.RepairOutcome, err, shiptest.HeadSHA(t, l.Repo), landed)
	}
}

func TestRun_AnIntentTheJournalDoesNotHoldUnwindsAndLandsThroughTheGates(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, shiptest.GithubServerError)
	if err := os.Remove(shipjournal.Path(paths.EvolveDirOf(l.Repo))); err != nil {
		t.Fatal(err)
	}

	res, err := runShip(t, l.Repo, l.options(sysexec.DefaultRunner, &shiptest.SleepLog{}))

	if err != nil || res.ExitCode != ship.ExitOK || shiptest.RemoteHeadSHA(t, l.Repo) != l.Tip(t) {
		t.Fatalf("exit=%d err=%v logs=%q: the lane unwinds and lands through its gates", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "the ship journal does not hold") || !containsLog(res, "ship unwound the lane") || !containsLog(res, "audit verified") {
		t.Errorf("outcome=%q logs=%q: an intent with no journal entry never resumes", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AnErrorInTheLaneTreeMeasureKeepsTheLaneRefAndStaysTransient(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	before := l.Tip(t)
	failed := false
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && !failed && len(args) > 0 && args[0] == "read-tree" {
			for _, e := range env {
				if strings.HasPrefix(e, "GIT_INDEX_FILE=") && strings.Contains(e, "evolve-lane-tree-") {
					failed = true
					_, _ = io.WriteString(stderr, "fatal: Unable to create index.lock: File exists.\n")
					return 128, nil
				}
			}
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}

	_, err := runShip(t, l.Repo, l.options(runner, &shiptest.SleepLog{}))

	if !failed {
		t.Fatal("the lane-tree measure never ran")
	}
	shiptest.WantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "pushed nothing")
	if got := l.Tip(t); got != before {
		t.Errorf("lane tip %s, want %s: a measure that fails before the unwind fields exist must not move the lane ref", got, before)
	}
}
