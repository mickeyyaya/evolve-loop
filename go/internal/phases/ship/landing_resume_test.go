//go:build integration

package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

const (
	githubServerError = "! [remote rejected] main -> main (Internal Server Error)\n"
	githubPolicyError = "! [remote rejected] main -> main (protected branch hook declined)\n"
	githubPushRace    = "! [rejected]        main -> main (fetch first)\n"
	landingCycle      = 1830
	landingRunID      = "run-1830"
	landingBranch     = "cycle-1830-branch"
	landingItemID     = "cli-phase-cycle-request"
	landingItem       = ".evolve/inbox/2026-09-30T10-51-14Z-cli-phase-cycle-request.json"
	landingGitignore  = ".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n"
)

type landingLane struct {
	repo, bare, wt, ws, base, audited string
	view                              *phaseio.ExplanationView
}

type pushOutage struct {
	failures int
	stderr   string
	pushes   int
	onPush   func()
}

func (o *pushOutage) runner() CmdRunner {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "push" {
			o.pushes++
			if o.onPush != nil {
				o.onPush()
			}
			if o.pushes <= o.failures {
				_, _ = io.WriteString(stderr, o.stderr)
				return 1, nil
			}
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

type sleepLog []time.Duration

func (s *sleepLog) sleep(d time.Duration) { *s = append(*s, d) }

func laneReadyToLand(t *testing.T) landingLane {
	t.Helper()
	l := landingLane{repo: makeRepo(t)}
	l.bare = addRemote(t, l.repo)
	mustWrite(t, filepath.Join(l.repo, ".gitignore"), landingGitignore)
	mustWrite(t, filepath.Join(l.repo, filepath.FromSlash(landingItem)), `{"id":"`+landingItemID+`","title":"cli phase cycle request"}`+"\n")
	runGit(t, l.repo, "add", ".gitignore", landingItem)
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "the lane's inbox item")
	runGit(t, l.repo, "push", "-q", "origin", "main")
	l.base = headSHA(t, l.repo)
	l.wt = makeWorktree(t, l.repo, landingBranch)
	l.ws = filepath.Join(l.repo, ".evolve", "runs", fmt.Sprintf("cycle-%d", landingCycle))
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), fmt.Sprintf(
		`{"cycle_id":%d,"run_id":%q,"audit_dispatches":1,"phase":"ship","active_worktree":%q,"workspace_path":%q,"worktree_base_sha":%q,"explanation_documentation_version":%d}`,
		landingCycle, landingRunID, l.wt, l.ws, l.base, explanationdocs.CurrentContractVersion))
	mustWrite(t, filepath.Join(l.ws, "triage-decision.json"), `{"schema_version":1,"top_n":[{"id":"`+landingItemID+`"}],"deferred":[],"dropped":[]}`)
	l.view = sealTheBuildExplanation(t, l)
	seedAudit(t, l.repo, "PASS")
	l.audited = strings.TrimSpace(runGitOut(t, l.wt, "write-tree"))
	return l
}

func sealTheBuildExplanation(t *testing.T, l landingLane) *phaseio.ExplanationView {
	t.Helper()
	binding := explanationdocs.CycleBinding{ProjectRoot: l.repo, Worktree: l.wt, Workspace: l.ws, BaseSHA: l.base,
		Cycle: landingCycle, RunID: landingRunID, ContractVersion: explanationdocs.CurrentContractVersion}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	document, err := explanationdocs.DocumentPath(landingCycle, landingRunID)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(l.wt, "fixture.txt"), "fixture line 1\nthe audited cycle work\n")
	mustWrite(t, filepath.Join(l.wt, filepath.FromSlash(document)), "# Build Explanation — Cycle 1830\n\n"+
		"## Build Binding\n- Cycle: 1830\n- Base SHA: "+l.base+"\n\n"+
		"## Summary\nThe fixture records the audited cycle work.\n\n"+
		"## Rationale\nThe tracked fixture is the smallest material change a landing can carry.\n\n"+
		"## Changed Areas\n- `fixture.txt` — adds the audited line the landing publishes.\n\n"+
		"## Design Decisions\nThe lane lands through the real worktree ship.\n\n"+
		"## Verification\nShip lands the audited commit across a failed push.\n\n"+
		"## Compatibility\nNo public interface changes.\n\n"+
		"## Limitations\nCovers the worktree landing only.\n")
	mustWrite(t, filepath.Join(l.ws, "build-report.md"), "## Explanation Documentation\n- Status: REQUIRED\n- Document: "+document+"\n")
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	view, err := explanationdocs.Load(l.ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	return view
}

func (l landingLane) options(runner CmdRunner, slept *sleepLog) Options {
	return Options{
		Class: ClassCycle, CommitMessage: "evolve-cycle 1830", CycleID: landingCycle, RunID: landingRunID, AuditRound: 1,
		ActiveWorktree: l.wt, WorktreeBaseSHA: l.base, WorkspacePath: l.ws,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.view,
		RequireBuildExplanationHandoff: true, Runner: runner, Sleep: slept.sleep,
	}
}

func (l landingLane) tip(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", landingBranch))
}

func (l landingLane) failEveryPush(t *testing.T, stderr string) (*pushOutage, sleepLog, RunResult, error) {
	t.Helper()
	outage := &pushOutage{failures: 1 << 20, stderr: stderr}
	var slept sleepLog
	res, err := runShip(t, l.repo, l.options(outage.runner(), &slept))
	if res.ExitCode == ExitOK {
		t.Fatalf("precondition: every push fails, so the first ship cannot land: %v", res.Logs)
	}
	return outage, slept, res, err
}

func (l landingLane) wantAuditedShape(t *testing.T) {
	t.Helper()
	head := strings.TrimSpace(runGitOut(t, l.wt, "rev-parse", "HEAD"))
	staged := strings.TrimSpace(runGitOut(t, l.wt, "write-tree"))
	if head != l.base || staged != l.audited {
		t.Errorf("lane HEAD=%s staged=%s, want the audited shape: HEAD=%s (the audited base) with the audited tree %s pending, so Audit never measures ship's own commit", head, staged, l.base, l.audited)
	}
}

func reauditTheLane(t *testing.T, l landingLane) {
	t.Helper()
	report := filepath.Join(l.ws, "audit-report.md")
	mustWrite(t, report, fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle %d (re-audit)\n\nVerdict: PASS\n\nA newer audit of the same tree.\n", landingCycle))
	sealTestPredicateEvidence(t, acssuite.EvidenceIdentity{Cycle: landingCycle, RunID: landingRunID, Round: 1, TreeSHA: l.audited}, report)
	row, err := json.Marshal(map[string]any{
		"ts": "2026-10-07T15:20:00Z", "cycle": landingCycle, "run_id": landingRunID, "worktree_tree_sha": l.audited,
		"role": "auditor", "kind": "agent_subprocess", "model": "sonnet", "exit_code": 0, "duration_s": "30",
		"artifact_path": report, "artifact_sha256": mustHashFile(t, report), "challenge_token": "testtoken123",
		"git_head": headSHA(t, l.repo), "tree_state_sha": treeStateSHA(t, l.repo), "worktree_base_sha": l.base,
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

func TestRun_ARetryAfterAFailedPushResumesAtThePushWithoutReverifying(t *testing.T) {
	l := laneReadyToLand(t)
	_, _, _, err := l.failEveryPush(t, githubServerError)
	wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "")
	landingCommit := l.tip(t)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("the retry after a failed push must land the prepared commit, got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairOutcome != "landing-resumed" || containsLog(res, "audit verified") {
		t.Errorf("repair_outcome=%q, audit re-verified=%v: the retry resumes at the push and re-runs no gate", res.RepairOutcome, containsLog(res, "audit verified"))
	}
	if got := remoteHeadSHA(t, l.repo); got != landingCommit || res.CommitSHA != landingCommit || headSHA(t, l.repo) != landingCommit {
		t.Errorf("origin=%s result=%s main=%s, want the prepared lane commit %s everywhere", got, res.CommitSHA, headSHA(t, l.repo), landingCommit)
	}
	binding, err := readStateMap(filepath.Join(l.ws, "ship-binding.json"))
	if err != nil || stateString(binding, "commit_sha") != landingCommit || stateString(binding, "audit_bound_tree_sha") != l.audited {
		t.Errorf("ship-binding.json = %v (err %v), want the landed commit %s bound to the audited tree %s", binding, err, landingCommit, l.audited)
	}
}

func TestRun_AFailedPushLeavesSharedMainAtItsPreLandingTip(t *testing.T) {
	l := laneReadyToLand(t)

	l.failEveryPush(t, githubServerError)

	if got := headSHA(t, l.repo); got != l.base {
		t.Errorf("shared main = %s after a failed push, want its pre-landing tip %s: a peer must never fork from or carry an unpushed landing", got, l.base)
	}
	if l.tip(t) == l.base {
		t.Errorf("the lane lost ship's commit; the prepared landing lives on the lane until the push lands")
	}
}

func TestRun_AFailedPushJournalsItsCommitSoPushOnlyCompletesIt(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	landingCommit := l.tip(t)
	runGit(t, l.repo, "merge", "--ff-only", "-q", landingBranch)

	res, err := runShip(t, l.repo, Options{PushOnly: true})

	if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != landingCommit {
		t.Fatalf("push-only = exit %d err %v, origin %s: a prepared landing is journaled before its push, so push-only completes it (want origin %s)", res.ExitCode, err, remoteHeadSHA(t, l.repo), landingCommit)
	}
}

func TestRun_AServerErrorTwiceIsRetriedWithBackoffInTheStep(t *testing.T) {
	l := laneReadyToLand(t)
	outage := &pushOutage{failures: 2, stderr: githubServerError}
	var slept sleepLog

	res, err := runShip(t, l.repo, l.options(outage.runner(), &slept))

	if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Fatalf("the live 2x500 shape must land in the same ship: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if want := (sleepLog{2 * time.Second, 4 * time.Second}); !reflect.DeepEqual(slept, want) || outage.pushes != 3 {
		t.Errorf("slept %v over %d pushes, want %v over 3: a transport error backs off and retries the identical push", slept, outage.pushes, want)
	}
}

func TestRun_AResumeDeclinesOnADivergedOriginAndUnwindsTheLaneToTheAuditedShape(t *testing.T) {
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	pushDivergentCommit(t, l.bare)
	diverged := remoteHeadSHA(t, l.repo)

	res, err := runShip(t, l.repo, l.options(execRunner, &sleepLog{}))

	se := wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassPrecondition, "")
	if se.Debug["repair_outcome"] != "needs-reaudit" || !containsLog(res, "does not resume: origin/main") {
		t.Errorf("Debug[repair_outcome] = %q, logs %q: the resume declines on the diverged origin before it pushes the stale commit", se.Debug["repair_outcome"], res.Logs)
	}
	l.wantAuditedShape(t)
	if status := l.intent(t)["status"]; status != "unwound" {
		t.Errorf("intent status %v, want unwound", status)
	}
	if remoteHeadSHA(t, l.repo) != diverged || headSHA(t, l.repo) != l.base {
		t.Errorf("origin=%s main=%s, want origin untouched at %s and main at its pre-landing tip %s", remoteHeadSHA(t, l.repo), headSHA(t, l.repo), diverged, l.base)
	}
}

func TestRun_APolicyRejectionIsNeverRetried(t *testing.T) {
	l := laneReadyToLand(t)

	outage, slept, _, err := l.failEveryPush(t, githubPolicyError)

	wantShipErr(t, err, "GIT_PUSH_POLICY_REFUSED", core.ShipClassPrecondition, "")
	if outage.pushes != 1 || len(slept) != 0 {
		t.Errorf("pushes=%d slept=%v, want exactly one push and no backoff: a policy rejection is final", outage.pushes, slept)
	}
	l.wantAuditedShape(t)
	if headSHA(t, l.repo) != l.base || l.intent(t)["status"] != "unwound" {
		t.Errorf("main = %s, intent %v: want main at its pre-landing tip %s and the intent unwound", headSHA(t, l.repo), l.intent(t)["status"], l.base)
	}
}

func TestRun_APushRaceIsRepairedWithoutBackoff(t *testing.T) {
	l := laneReadyToLand(t)
	outage := &pushOutage{failures: 1, stderr: githubPushRace}
	var slept sleepLog

	res, err := runShip(t, l.repo, l.options(outage.runner(), &slept))

	if err != nil || res.ExitCode != ExitOK || remoteHeadSHA(t, l.repo) != l.tip(t) {
		t.Fatalf("a race whose origin is an ancestor re-pushes after a fetch: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if len(slept) != 0 || res.RepairOutcome != "push-retried" {
		t.Errorf("slept=%v outcome=%q, want no backoff and push-retried: only transport errors back off", slept, res.RepairOutcome)
	}
}

func TestRun_ALandingIntentThatCannotBeRecordedUnwindsAndNeverPushes(t *testing.T) {
	l := laneReadyToLand(t)
	l.lockTheLandingDir(t)
	outage := &pushOutage{}

	_, err := runShip(t, l.repo, l.options(outage.runner(), &sleepLog{}))

	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "ship: ship cannot record the landing intent, so it pushed nothing")
	if outage.pushes != 0 || remoteHeadSHA(t, l.repo) != l.base || headSHA(t, l.repo) != l.base {
		t.Errorf("pushes=%d origin=%s main=%s: without a durable intent nothing is pushed and nothing moves", outage.pushes, remoteHeadSHA(t, l.repo), headSHA(t, l.repo))
	}
	l.wantAuditedShape(t)
}

func TestRun_ALandingThatCannotUnwindStopsTheCycleWithItsCommitJournaled(t *testing.T) {
	l := laneReadyToLand(t)
	outage := &pushOutage{failures: 1, stderr: githubPolicyError, onPush: func() {
		mustWrite(t, filepath.Join(l.wt, "debris.txt"), "a writer ship did not expect\n")
	}}

	_, err := runShip(t, l.repo, l.options(outage.runner(), &sleepLog{}))

	se := wantShipErr(t, err, core.CodeGitLandingUnwindDeclined, core.ShipClassIntegrity, "ship cannot unwind the lane to the audited shape")
	stranded := l.tip(t)
	if stranded == l.base || se.Debug["commit"] != stranded || !journalHasSHA(l.repo, stranded) {
		t.Errorf("lane tip %s, Debug %v, journaled=%v: the commit stays on the lane and journaled for the boundary", stranded, se.Debug, journalHasSHA(l.repo, stranded))
	}
	for _, step := range []string{"1. In the plane, run `git merge --ff-only " + stranded + "`", "2. Run `evolve sync-main`", "3. Run `evolve ship --push-only`"} {
		if !strings.Contains(se.Message, step) {
			t.Errorf("message %q lacks the operator step %q", se.Message, step)
		}
	}
	if headSHA(t, l.repo) != l.base || remoteHeadSHA(t, l.repo) != l.base {
		t.Errorf("main=%s origin=%s, want both at %s", headSHA(t, l.repo), remoteHeadSHA(t, l.repo), l.base)
	}
}
