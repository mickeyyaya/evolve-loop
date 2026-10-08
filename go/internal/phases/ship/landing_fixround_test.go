//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func (l landingLane) intentFile() string {
	return filepath.Join(l.repo, ".evolve", "landing", fmt.Sprintf("cycle-%d.json", landingCycle))
}

func (l landingLane) lockTheLandingDir(t *testing.T) {
	t.Helper()
	dir := filepath.Dir(l.intentFile())
	mustMkdir(t, dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func (l landingLane) writeIntentMap(t *testing.T, in map[string]any) {
	t.Helper()
	body, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, l.intentFile(), string(body)+"\n")
}

func readIntentAt(t *testing.T, path string) (map[string]any, bool) {
	t.Helper()
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal(body, &in); err != nil {
		t.Fatal(err)
	}
	return in, true
}

func (l landingLane) intent(t *testing.T) map[string]any {
	t.Helper()
	in, found := readIntentAt(t, l.intentFile())
	if !found {
		t.Fatalf("no landing intent at %s", l.intentFile())
	}
	return in
}

func (l landingLane) rewindToPrepared(t *testing.T) {
	t.Helper()
	l.setIntentField(t, "status", "prepared")
	if err := os.Remove(filepath.Join(l.ws, "ship-binding.json")); err != nil {
		t.Fatal(err)
	}
}

func (l *landingLane) pendTheAuditedChangeAgain(t *testing.T) {
	t.Helper()
	runGit(t, l.wt, "reset", "-q", "--hard", l.base)
	runGit(t, l.wt, "read-tree", "-m", "-u", l.base, l.audited)
}

func (l *landingLane) rebuildWithAnotherChange(t *testing.T) {
	t.Helper()
	l.pendTheAuditedChangeAgain(t)
	mustWrite(t, filepath.Join(l.wt, "fixture.txt"), "fixture line 1\nthe audited cycle work, revised by a repair round\n")
	binding := explanationdocs.CycleBinding{ProjectRoot: l.repo, Worktree: l.wt, Workspace: l.ws, BaseSHA: l.base,
		Cycle: landingCycle, RunID: landingRunID, ContractVersion: explanationdocs.CurrentContractVersion}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	view, err := explanationdocs.Load(l.ws)
	if err != nil {
		t.Fatal(err)
	}
	l.view = view
	runGit(t, l.wt, "add", "-A")
	l.audited = strings.TrimSpace(runGitOut(t, l.wt, "write-tree"))
}

func appendAnAuditOfAnotherRun(t *testing.T, l landingLane) {
	t.Helper()
	report := filepath.Join(t.TempDir(), "audit-report.md")
	mustWrite(t, report, "# Audit Report — another run\n\nVerdict: PASS\n")
	row, err := json.Marshal(map[string]any{
		"ts": "2026-10-07T15:30:00Z", "cycle": landingCycle, "run_id": "run-1830-other", "worktree_tree_sha": l.base,
		"role": "auditor", "kind": "agent_subprocess", "model": "sonnet", "exit_code": 0, "duration_s": "30",
		"artifact_path": report, "artifact_sha256": mustHashFile(t, report), "git_head": l.base, "tree_state_sha": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(l.repo, ".evolve", "ledger.jsonl")
	prior, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, ledger, string(prior)+string(row)+"\n")
}

func failFastForwards(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if name == "git" && len(args) > 1 && args[0] == "merge" && args[1] == "--ff-only" {
		_, _ = io.WriteString(stderr, "fatal: Unable to create '.git/index.lock': File exists.\n")
		return 128, nil
	}
	return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
}

func hasRef(repo, ref string) bool {
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", ref)
	cmd.Env = filteredEnv()
	return cmd.Run() == nil
}

func pushOnTopOf(t *testing.T, bare, commit string) string {
	t.Helper()
	clone := filepath.Join(tempRepoDir(t), "peer")
	runGit(t, filepath.Dir(clone), "clone", "-q", bare, clone)
	runGit(t, clone, "checkout", "-q", "-B", "main", commit)
	runGit(t, clone, "config", "user.email", "peer@evolve-loop.test")
	runGit(t, clone, "config", "user.name", "Peer Lane")
	mustWrite(t, filepath.Join(clone, "peer.txt"), "a peer landing on top\n")
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "peer landing")
	runGit(t, clone, "push", "-q", "origin", "main")
	return strings.TrimSpace(runGitOut(t, clone, "rev-parse", "HEAD"))
}

func TestRun_ASupersedingAuditOfAnotherTreeMarksTheIntentStaleAndTheGatesBindIt(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	stale := l.tip(t)
	l.rebuildWithAnotherChange(t)
	reauditTheLane(t, l)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("a newer audit of another tree re-ships through the gates, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	binding, berr := readStateMap(filepath.Join(l.ws, "ship-binding.json"))
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "audit verified") || berr != nil ||
		stateString(binding, "commit_sha") == stale || stateString(binding, "commit_sha") != remoteHeadSHA(t, l.repo) ||
		stateString(binding, "audit_bound_tree_sha") != l.audited {
		t.Errorf("outcome=%q binding=%v: the newer audit binds the landing, never the stale commit %s", res.RepairOutcome, binding, stale)
	}
}

func TestRun_ASupersededIntentOnItsOwnCommitUnwindsBeforeTheGates(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	reauditTheLane(t, l)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("a superseded intent on its own commit unwinds and re-ships, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "does not resume: a newer audit") ||
		!containsLog(res, "ship unwound the lane") || !containsLog(res, "audit verified") || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Errorf("outcome=%q logs=%q: the lane unwinds to the audited shape, the gates bind the newer audit, and the landing completes", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AResumeSettlesACommitOriginHoldsUnderAPeerLanding(t *testing.T) {
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{})); err != nil || res.ExitCode != ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.tip(t)
	l.rewindToPrepared(t)
	peer := pushOnTopOf(t, l.bare, landed)
	runGit(t, l.repo, "fetch", "-q", "origin", "main")
	runGit(t, l.repo, "merge", "--ff-only", "-q", "origin/main")

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("origin already holds %s under the peer %s, so the resume settles it: exit=%d err=%v logs=%v", landed, peer, res.ExitCode, err, res.Logs)
	}
	binding, _ := readStateMap(filepath.Join(l.ws, "ship-binding.json"))
	if stateString(binding, "commit_sha") != landed || headSHA(t, l.repo) != peer || remoteHeadSHA(t, l.repo) != peer || l.intent(t)["status"] != "complete" {
		t.Errorf("binding=%v main=%s origin=%s intent=%v: no push, no unwind, the binding names %s and the intent completes", binding, headSHA(t, l.repo), remoteHeadSHA(t, l.repo), l.intent(t)["status"], landed)
	}
}

func TestRun_AResumeFastForwardsMainToACommitAPeerPushedOnTopOf(t *testing.T) {
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{})); err != nil || res.ExitCode != ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.tip(t)
	l.rewindToPrepared(t)
	runGit(t, l.repo, "reset", "-q", "--hard", l.base)
	peer := pushOnTopOf(t, l.bare, landed)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("a kill after the push left main at %s; origin holds %s under the peer %s: exit=%d err=%v logs=%v", l.base, landed, peer, res.ExitCode, err, res.Logs)
	}
	if headSHA(t, l.repo) != landed || remoteHeadSHA(t, l.repo) != peer || l.intent(t)["status"] != "complete" {
		t.Errorf("main=%s origin=%s intent=%v: main fast-forwards to the landed %s, origin stays at the peer", headSHA(t, l.repo), remoteHeadSHA(t, l.repo), l.intent(t)["status"], landed)
	}
}

func TestRun_ALaggingMainCatchesUpBeforeTheNextLandingChecksItsFastForward(t *testing.T) {
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.repo, l.options(failFastForwards, &sleepLog{})); err != nil || res.ExitCode != ExitOK {
		t.Fatalf("precondition: the first landing pushes and only its fast-forward fails: %v %v", err, res.Logs)
	}
	landed := remoteHeadSHA(t, l.repo)
	next := makeWorktree(t, l.repo, "cycle-1831-branch")
	mustWrite(t, filepath.Join(next, "next.txt"), "the next lane's work\n")
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	seedAudit(t, l.repo, "PASS")
	outage := &pushOutage{}

	_, err := runShip(t, l.repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1831", Runner: outage.runner(), Env: map[string]string{"EVOLVE_FLEET": "1"}})

	wantShipErr(t, err, core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	if headSHA(t, l.repo) != landed || outage.pushes != 0 || remoteHeadSHA(t, l.repo) != landed {
		t.Errorf("main=%s pushes=%d origin=%s: main catches up to the journaled %s first, so the next lane's divergence is a fleet rebase, not a push race", headSHA(t, l.repo), outage.pushes, remoteHeadSHA(t, l.repo), landed)
	}
}

func TestRun_AForgedIntentNeverShipsAnAuditFAIL(t *testing.T) {
	l := laneReadyToLand(t)
	seedAudit(t, l.repo, "FAIL")
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "not a ship commit")
	forged := l.tip(t)
	tree := strings.TrimSpace(runGitOut(t, l.wt, "rev-parse", "HEAD^{tree}"))
	view, err := explanationViewSHA(l.repo, landingCycle, landingRunID)
	if err != nil {
		t.Fatal(err)
	}
	l.writeIntentMap(t, map[string]any{"cycle": landingCycle, "run_id": landingRunID, "audit_artifact_sha256": mustHashFile(t, filepath.Join(l.ws, "audit-report.md")),
		"audited_tree": tree, "lane_tree": tree, "worktree_base_sha": l.base, "commit_sha": forged, "commit_tree": tree,
		"explanation_view_sha256": view, "pre_main": l.base, "branch": "main", "status": "prepared"})

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err == nil || res.ExitCode == ExitOK || remoteHeadSHA(t, l.repo) == forged {
		t.Errorf("exit=%d origin=%s: an intent that no ship journaled, over an audit FAIL, never ships %s", res.ExitCode, remoteHeadSHA(t, l.repo), forged)
	}
}

func (l landingLane) setIntentField(t *testing.T, key string, value any) {
	t.Helper()
	in := l.intent(t)
	in[key] = value
	l.writeIntentMap(t, in)
}

func TestRun_AResumeTakesTheBranchFromTheHostNeverFromTheIntent(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	l.setIntentField(t, "branch", "forged")

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || remoteHeadSHA(t, l.repo) != l.tip(t) || hasRef(l.bare, "refs/heads/forged") {
		t.Errorf("outcome=%q err=%v origin=%s forged-ref=%v: the resume pushes the host's branch main, whatever branch the intent names", res.RepairOutcome, err, remoteHeadSHA(t, l.repo), hasRef(l.bare, "refs/heads/forged"))
	}
}

func TestRun_APreparedIntentOfAnotherRunNeverStopsAPendingLane(t *testing.T) {
	for name, options := range map[string]func(landingLane) Options{
		"the phase path names the run": func(l landingLane) Options { return l.options(execRunner, &sleepLog{}) },
		"the CLI path names no run": func(landingLane) Options {
			return Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1830", Runner: execRunner}
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := laneReadyToLand(t)
			l.writeIntentMap(t, map[string]any{"cycle": landingCycle, "run_id": "run-older", "audit_artifact_sha256": strings.Repeat("a", 64),
				"audited_tree": l.audited, "lane_tree": l.audited, "worktree_base_sha": l.base, "commit_sha": l.base, "commit_tree": l.audited,
				"pre_main": l.base, "branch": "main", "status": "prepared"})

			res, err := runShip(t, l.repo, options(l))

			if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != l.tip(t) {
				t.Errorf("exit=%d err=%v logs=%q: the witness takes the run from the host, so an intent of another run is stale and the lane ships through its gates", res.ExitCode, err, res.Logs)
			}
		})
	}
}

func TestRun_AResumeNeverUnwindsALaneTipShipDidNotPrepare(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "--amend", "-m", "a lane tip ship did not prepare")
	moved := l.tip(t)

	res, _ := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if l.tip(t) != moved || containsLog(res, "ship unwound the lane") || remoteHeadSHA(t, l.repo) == moved {
		t.Errorf("lane=%s logs=%q: ship never unwinds or pushes a lane tip %s that it did not prepare", l.tip(t), res.Logs, moved)
	}
	if status := l.intent(t)["status"]; status != "stale" {
		t.Errorf("intent status %v, want stale", status)
	}
}

func TestRun_AResumeAfterTheOrchestratorPendedTheLaneMarksTheIntentStale(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	l.pendTheAuditedChangeAgain(t)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Fatalf("a lane pended again by the orchestrator ships through its gates, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "does not resume: the lane tip moved") || containsLog(res, "ship unwound the lane") {
		t.Errorf("outcome=%q logs=%q: the moved lane is not unwound; the intent goes stale", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AFailedWriteOfTheUnwoundIntentIsAnError(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	pushDivergentCommit(t, l.bare)
	l.lockTheLandingDir(t)

	_, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "landing intent unwound")
	l.wantAuditedShape(t)
}

func TestRun_AResumeBindsTheNewestAuditOfItsOwnRun(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	appendAnAuditOfAnotherRun(t, l)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" {
		t.Errorf("outcome=%q err=%v: a newer audit of another run never supersedes this run's landing", res.RepairOutcome, err)
	}
}

func TestRun_AKillBeforeTheLaneRefMovesRollsTheLaneRefForward(t *testing.T) {
	l := laneReadyToLand(t)
	killed := false
	kill := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && !killed && hasArg(args, "update-ref") {
			killed = true
			return 1, nil
		}
		if name == "git" && len(args) > 0 && args[0] == "push" {
			_, _ = io.WriteString(stderr, githubServerError)
			return 1, nil
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	if res, _ := runShip(t, l.repo, l.options(kill, &sleepLog{})); res.ExitCode == ExitOK {
		t.Fatalf("precondition: the first ship stops before it lands: %v", res.Logs)
	}

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || !containsLog(res, "moves the lane ref forward") || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Errorf("exit=%d outcome=%q err=%v logs=%q: the intent and the journal exist before the lane ref moves, so the retry moves the ref forward and resumes", res.ExitCode, res.RepairOutcome, err, res.Logs)
	}
}

func TestRun_APeerLaneOnAStrandedLandingStopsWithItsSteps(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	runGit(t, l.repo, "merge", "--ff-only", "-q", landingBranch)
	next := makeWorktree(t, l.repo, "cycle-1831-branch")
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	seedAudit(t, l.repo, "PASS")

	res, err := runShip(t, l.repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1831"})

	se := wantShipErr(t, err, "GIT_LANE_NOT_ON_ORIGIN", core.ShipClassIntegrity, "")
	for _, step := range []string{"1. In the plane, run `git merge --ff-only " + l.tip(t) + "`", "2. Run `evolve sync-main`", "3. Run `evolve ship --push-only`"} {
		if !strings.Contains(se.Message, step) {
			t.Errorf("message %q lacks the step %q", se.Message, step)
		}
	}
	if res.ExitCode == ExitOK {
		t.Error("a lane on a stranded landing never passes with no push")
	}
}

func TestRun_ALaneOnAMainAheadByBoundaryCommitsIsStillNothingToShip(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	published := headSHA(t, repo)
	mustWrite(t, filepath.Join(repo, "docs", "dossier.md"), "a dossier closeout\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "dossier: cycle-1830 closeout")
	wt := makeWorktree(t, repo, "cycle-1831-branch")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1"})

	if err != nil || res.ExitCode != ExitOK || !containsLog(res, "exiting cleanly") || remoteHeadSHA(t, repo) != published {
		t.Errorf("exit=%d err=%v: a boundary commit ahead of origin is no stranded landing, so a lane with nothing to ship exits cleanly", res.ExitCode, err)
	}
}

func TestRun_ALaneOnACompletedLandingOriginLostIsStillNothingToShip(t *testing.T) {
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{})); err != nil || res.ExitCode != ExitOK {
		t.Fatalf("precondition: the landing completes: %v %v", err, res.Logs)
	}
	runGit(t, l.bare, "update-ref", "refs/heads/main", l.base)
	runGit(t, l.repo, "update-ref", "refs/remotes/origin/main", l.base)
	next := makeWorktree(t, l.repo, "cycle-1831-branch")
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1831,"run_id":"run-1831","audit_dispatches":1,"phase":"ship","active_worktree":"`+next+`"}`)
	seedAudit(t, l.repo, "PASS")

	res, err := runShip(t, l.repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1831"})

	if err != nil || res.ExitCode != ExitOK || !containsLog(res, "exiting cleanly") {
		t.Errorf("exit=%d err=%v: a journaled lane commit whose intent is complete is no stranded landing", res.ExitCode, err)
	}
}

func (l landingLane) carryOntoAPeer(t *testing.T) (string, string) {
	t.Helper()
	base1 := pushOnTopOf(t, l.bare, l.base)
	runGit(t, l.repo, "fetch", "-q", "origin", "main")
	runGit(t, l.repo, "merge", "--ff-only", "-q", "origin/main")
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "the audited lane change")
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "rebase", "-q", base1)
	runGit(t, l.wt, "reset", "-q", "--soft", base1)
	tree1 := strings.TrimSpace(runGitOut(t, l.wt, "write-tree"))
	writeCarry(t, carriedLane{repo: l.repo, worktree: l.wt, base0: l.base, tree0: l.audited, base1: base1, tree1: tree1, cycle: landingCycle},
		mustHashFile(t, filepath.Join(l.ws, "audit-report.md")), l.audited)
	return base1, tree1
}

func (l landingLane) prepareACarriedLanding(t *testing.T, base1, tree1 string) string {
	t.Helper()
	consumed := ".evolve/inbox/consumed/" + filepath.Base(landingItem)
	mustWrite(t, filepath.Join(l.wt, filepath.FromSlash(consumed)), `{"id":"`+landingItemID+`","consumed":{"via":"ship"}}`+"\n")
	runGit(t, l.wt, "rm", "-q", landingItem)
	runGit(t, l.wt, "add", consumed)
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "evolve-cycle 1830")
	commit := l.tip(t)
	journal, err := json.Marshal(map[string]any{"sha": commit, "class": "cycle", "ts": "2026-10-08T00:00:00Z", "cycle": landingCycle})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(l.repo, ".evolve", "ship-journal.jsonl"), string(journal)+"\n")
	view, err := explanationViewSHA(l.repo, landingCycle, landingRunID)
	if err != nil {
		t.Fatal(err)
	}
	l.writeIntentMap(t, map[string]any{"cycle": landingCycle, "run_id": landingRunID,
		"audit_artifact_sha256": mustHashFile(t, filepath.Join(l.ws, "audit-report.md")), "audited_tree": l.audited, "lane_tree": tree1,
		"worktree_base_sha": base1, "commit_sha": commit, "commit_tree": strings.TrimSpace(runGitOut(t, l.wt, "rev-parse", "HEAD^{tree}")),
		"consumed_paths": []string{landingItem, consumed}, "explanation_view_sha256": view, "pre_main": base1,
		"lane_branch": landingBranch, "branch": "main", "status": "prepared"})
	return commit
}

func TestRun_APolicyRefusalOfACarriedLaneUnwindsItToItsCarriedTree(t *testing.T) {
	l := laneReadyToLand(t)
	base1, tree1 := l.carryOntoAPeer(t)
	l.prepareACarriedLanding(t, base1, tree1)
	outage := &pushOutage{failures: 1 << 20, stderr: githubPolicyError}

	_, err := runShip(t, l.repo, l.options(outage.runner(), &sleepLog{}))

	wantShipErr(t, err, "GIT_PUSH_POLICY_REFUSED", core.ShipClassPrecondition, "")
	head := strings.TrimSpace(runGitOut(t, l.wt, "rev-parse", "HEAD"))
	staged := strings.TrimSpace(runGitOut(t, l.wt, "write-tree"))
	if head != base1 || staged != tree1 {
		t.Errorf("lane HEAD=%s staged=%s, want the carried change %s pending on the peer's base %s: the unwind restores the lane's own tree, not the tree Audit saw on the old base", head, staged, tree1, base1)
	}
	if status := l.intent(t)["status"]; status != "unwound" {
		t.Errorf("intent status %v, want unwound", status)
	}
}

func TestRun_AnUnreadableCycleStateStopsTheResumeLoudly(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), "{")

	_, err := runShip(t, l.repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1830", Runner: execRunner})

	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "landing intent")
}

func TestRun_AStopAfterThePushAndBeforeTheFastForwardSettlesWithNoPush(t *testing.T) {
	l := laneReadyToLand(t)
	if res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{})); err != nil || res.ExitCode != ExitOK {
		t.Fatalf("precondition: the first landing completes: %v %v", err, res.Logs)
	}
	landed := l.tip(t)
	l.rewindToPrepared(t)
	runGit(t, l.repo, "reset", "-q", "--hard", l.base)
	outage := &pushOutage{}

	res, err := runShip(t, l.repo, l.options(outage.runner(), &sleepLog{}))

	if err != nil || res.RepairOutcome != "landing-resumed" || headSHA(t, l.repo) != landed || l.intent(t)["status"] != "complete" {
		t.Errorf("outcome=%q err=%v main=%s: the resume settles the pushed %s and fast-forwards main to it", res.RepairOutcome, err, headSHA(t, l.repo), landed)
	}
}

func TestRun_AnIntentTheJournalDoesNotHoldUnwindsAndLandsThroughTheGates(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	if err := os.Remove(shipJournalPath(l.repo)); err != nil {
		t.Fatal(err)
	}

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Fatalf("exit=%d err=%v logs=%q: the lane unwinds and lands through its gates", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome == "landing-resumed" || !containsLog(res, "the ship journal does not hold") || !containsLog(res, "ship unwound the lane") || !containsLog(res, "audit verified") {
		t.Errorf("outcome=%q logs=%q: an intent with no journal entry never resumes", res.RepairOutcome, res.Logs)
	}
}

func TestRun_AnErrorInTheLaneTreeMeasureKeepsTheLaneRefAndStaysTransient(t *testing.T) {
	l := laneReadyToLand(t)
	before := l.tip(t)
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
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}

	_, err := runShip(t, l.repo, l.options(runner, &sleepLog{}))

	if !failed {
		t.Fatal("the lane-tree measure never ran")
	}
	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "pushed nothing")
	if got := l.tip(t); got != before {
		t.Errorf("lane tip %s, want %s: a measure that fails before the unwind fields exist must not move the lane ref", got, before)
	}
}
