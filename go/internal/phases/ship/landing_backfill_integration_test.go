//go:build integration

package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

func (l landingLane) worktreeShip(runner CmdRunner) (*worktreeShip, *Options, *RunResult) {
	opts := l.options(runner, &sleepLog{})
	opts.ProjectRoot, opts.PluginRoot, opts.ShipBinaryPath = l.Repo, l.Repo, filepath.Join(l.Repo, "ship-binary-fixture")
	opts.Stdout, opts.Stderr = io.Discard, io.Discard
	res := &RunResult{}
	s := newWorktreeShip(context.Background(), &opts, res, "main", l.WT)
	s.cycleBranch = landingBranch
	return s, &opts, res
}

func (l landingLane) preparedIntent(t *testing.T) landing.Intent {
	t.Helper()
	l.failEveryPush(t, githubServerError)
	opts := l.options(execRunner, &sleepLog{})
	opts.ProjectRoot = l.Repo
	in, found, err := readLandingIntent(&opts)
	if err != nil || !found || in.Status != landing.IntentPrepared {
		t.Fatalf("precondition: a failed push leaves a prepared intent, got %+v found=%v err=%v", in, found, err)
	}
	return in
}

func realGit(when argvMatch, exit int) *gitFault {
	return &gitFault{when: when, exit: exit, next: execRunner}
}

func pushed(calls [][]string) int {
	n := 0
	for _, c := range calls {
		if sub := gitSubArgs(c); len(sub) > 0 && sub[0] == "push" {
			n++
		}
	}
	return n
}

func TestLand_AnUnreadableLaneTipStopsBeforeTheJournalOrTheIntent(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	fault := realGit(argvIs("rev-parse", "HEAD"), 128)
	s, _, _ := l.worktreeShip(fault.runner())

	err := s.land("")

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "exited 128")
	if journal := readShipJournal(l.Repo); len(journal) != 0 || pushed(fault.calls) != 0 {
		t.Errorf("journal %v, pushes %d: an unknown lane tip journals and pushes nothing", journal, pushed(fault.calls))
	}
	if _, found := readIntentAt(t, l.IntentFile()); found || l.Tip(t) != l.Base {
		t.Errorf("intent written=%v lane tip %s: nothing is prepared and the lane stays at %s", found, l.Tip(t), l.Base)
	}
}

func TestLand_ADivergedMainWithAStuckLaneRefReportsTheRefThatDidNotMove(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	tree := strings.TrimSpace(runGitOut(t, l.WT, "write-tree"))
	pending := strings.TrimSpace(runGitOut(t, l.WT, "-c", "commit.gpgsign=false", "commit-tree", tree, "-p", "HEAD", "-m", "evolve-cycle 1830"))
	mustWrite(t, filepath.Join(l.Repo, "boundary.txt"), "a boundary commit main gained\n")
	runGit(t, l.Repo, "add", "boundary.txt")
	runGit(t, l.Repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "boundary")
	fault := realGit(argvStarts("update-ref"), 1)
	s, _, _ := l.worktreeShip(fault.runner())

	err := s.land(pending)

	se := wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "the lane ref did not move")
	if se.Debug["commit"] != pending || fault.hits != 1 || l.Tip(t) != l.Base || pushed(fault.calls) != 0 {
		t.Errorf("debug %v, update-ref attempts %d, lane %s, pushes %d: the refusal names the commit whose ref did not move, and nothing is pushed",
			se.Debug, fault.hits, l.Tip(t), pushed(fault.calls))
	}
}

func TestAbandonPrepared_AnUnwindThatCannotRunStopsWithTheBoundarySteps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	gone := filepath.Join(t.TempDir(), "removed-lane")
	s := newWorktreeShip(context.Background(), &Options{ProjectRoot: root, Runner: execRunner}, &RunResult{}, "main", gone)
	in := landing.Intent{Cycle: 1830, RunID: landingRunID, CommitSHA: strings.Repeat("c", 40), Branch: "main",
		WorktreeBaseSHA: strings.Repeat("b", 40), LaneTree: strings.Repeat("e", 40)}
	cause := errors.New("the journal is full")

	err := s.abandonPrepared("", in, cause)

	se := wantShipErr(t, err, core.CodeGitLandingUnwindDeclined, core.ShipClassIntegrity, "cannot complete (the journal is full)")
	if !strings.Contains(se.Debug["unwind_declined"], "read the worktree status") || !strings.Contains(se.Message, "3. Run `evolve ship --push-only`") || se.Debug["worktree"] != gone {
		t.Errorf("debug %v message %q: an unwind that cannot read the lane is reported with the boundary steps", se.Debug, se.Message)
	}
	if errors.Is(err, cause) {
		t.Errorf("err %v carries the cause as its identity; the unwind refusal supersedes it", err)
	}
}

func TestDecline_ARunOwnedTipThatCannotUnwindKeepsTheIntentPrepared(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opts := &Options{ProjectRoot: root, Runner: execRunner}
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", filepath.Join(t.TempDir(), "removed-lane"))
	in := landing.Intent{Cycle: 1830, RunID: landingRunID, CommitSHA: strings.Repeat("c", 40), Branch: "main",
		WorktreeBaseSHA: strings.Repeat("b", 40), LaneTree: strings.Repeat("e", 40), Status: landing.IntentPrepared}
	w := landing.Witness{Cycle: in.Cycle, RunID: in.RunID, LaneTip: in.CommitSHA}

	err := s.decline(in, w, "origin/main diverged")

	wantShipErr(t, err, core.CodeGitLandingUnwindDeclined, core.ShipClassIntegrity, "cannot complete (origin/main diverged)")
	if _, found := readIntentAt(t, intentPath(opts, in.Cycle)); found {
		t.Errorf("an intent was written: a lane that cannot unwind is neither stale nor unwound")
	}
	if !containsLog(*s.result, "does not resume: origin/main diverged") || containsLog(*s.result, "marks the intent stale") {
		t.Errorf("logs %q: the decline is logged and the run-owned tip is not treated as stale", s.result.Logs)
	}
}

func TestSettleLanding_EachSettleStepFailsOnItsOwnTerms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		arrange   func(t *testing.T, l landingLane, opts *Options)
		wantErr   core.ShipErrorCode
		wantWarn  string
		wantState landing.IntentStatus
	}{
		{name: "a committed tree the audit never bound refuses the landing", wantErr: core.CodeIntegrityTreeDrift, wantState: landing.IntentPrepared,
			arrange: func(t *testing.T, l landingLane, opts *Options) {
				opts.internalAuditBoundTreeSHA = strings.TrimSpace(runGitOut(t, l.Repo, "rev-parse", l.Base+"^{tree}"))
			}},
		{name: "a binding that cannot be written only warns", wantWarn: "[ship] WARN: could not write ship-binding.json: no cycle_id in cycle-state.json", wantState: landing.IntentComplete,
			arrange: func(t *testing.T, l landingLane, opts *Options) {
				opts.CycleID = 0
				mustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), `{"run_id":"`+landingRunID+`"}`)
			}},
		{name: "an intent that cannot be marked complete only warns", wantWarn: "[ship] WARN: ship cannot mark the landing intent complete", wantState: landing.IntentPrepared,
			arrange: func(t *testing.T, l landingLane, opts *Options) {
				skipWhenRoot(t)
				l.LockTheLandingDir(t)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := laneReadyToLand(t)
			in := l.preparedIntent(t)
			_, opts, res := l.worktreeShip(execRunner)
			tc.arrange(t, l, opts)

			err := settleLanding(context.Background(), opts, res, in)

			if tc.wantErr != "" {
				wantShipErr(t, err, tc.wantErr, core.ShipClassIntegrity, "worktree-to-main tree drift detected")
				if res.CommitSHA != "" {
					t.Errorf("CommitSHA = %q after a refused settle", res.CommitSHA)
				}
			} else if err != nil || res.CommitSHA != in.CommitSHA || !containsLog(*res, tc.wantWarn) {
				t.Errorf("err=%v commit=%q logs=%q: the landing settles with the WARN %q", err, res.CommitSHA, res.Logs, tc.wantWarn)
			}
			if status := landing.IntentStatus(l.Intent(t)["status"].(string)); status != tc.wantState {
				t.Errorf("intent status %q, want %q", status, tc.wantState)
			}
		})
	}
}

func TestResumeLanding_DeclinesToResumeWithoutAHostWorktreeBranchOrLock(t *testing.T) {
	t.Parallel()
	noFault := func() *gitFault { return realGit(func([]string) bool { return false }, 0) }
	cases := []struct {
		name     string
		arrange  func(l landingLane, opts *Options) *gitFault
		wantCode core.ShipErrorCode
		wantErr  string
	}{
		{name: "the host worktree is gone", wantCode: core.CodeWorktreeResolve, wantErr: "is unavailable",
			arrange: func(l landingLane, opts *Options) *gitFault {
				opts.ActiveWorktree = filepath.Join(l.Repo, "no-such-lane")
				return noFault()
			}},
		{name: "a ship from the project root",
			arrange: func(l landingLane, opts *Options) *gitFault {
				opts.ActiveWorktree = l.Repo
				return noFault()
			}},
		{name: "a detached host HEAD",
			arrange: func(landingLane, *Options) *gitFault { return realGit(argvIs("symbolic-ref", "--short", "HEAD"), 1) }},
		{name: "an unreadable host branch", wantErr: "ship: symbolic-ref --short HEAD: git vanished",
			arrange: func(landingLane, *Options) *gitFault {
				f := realGit(argvIs("symbolic-ref", "--short", "HEAD"), 0)
				f.err = errors.New("git vanished")
				return f
			}},
		{name: "the integrator lock is held", wantCode: core.CodeGitIO, wantErr: "acquire integrator lock (.evolve/ship.lock): held by a peer",
			arrange: func(_ landingLane, opts *Options) *gitFault {
				opts.shipLock = func(string) (func(), error) { return nil, errors.New("held by a peer") }
				return noFault()
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := laneReadyToLand(t)
			l.preparedIntent(t)
			_, opts, res := l.worktreeShip(nil)
			fault := tc.arrange(l, opts)
			opts.Runner = fault.runner()

			resumed, err := resumeLanding(context.Background(), opts, res)

			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
			if tc.wantCode != "" {
				if se := mustShipErr(t, err); se.Code != tc.wantCode {
					t.Errorf("code = %s, want %s", se.Code, tc.wantCode)
				}
			}
			if resumed || pushed(fault.calls) != 0 || remoteHeadSHA(t, l.Repo) != l.Base || l.Intent(t)["status"] != "prepared" {
				t.Errorf("resumed=%v pushes=%d origin=%s intent=%v: nothing resumes and the intent stays prepared", resumed, pushed(fault.calls), remoteHeadSHA(t, l.Repo), l.Intent(t)["status"])
			}
		})
	}
}

func TestRun_ARollForwardWhoseLaneRefCannotMoveStopsTheResume(t *testing.T) {
	t.Parallel()
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
	if res, _ := runShip(t, l.Repo, l.options(kill, &sleepLog{})); res.ExitCode == ExitOK {
		t.Fatalf("precondition: the first ship stops before the lane ref moves: %v", res.Logs)
	}
	prepared := l.Intent(t)["commit_sha"]
	fault := realGit(argvStarts("update-ref"), 1)

	res, err := runShip(t, l.Repo, l.options(fault.runner(), &sleepLog{}))

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "the lane ref did not move")
	if l.Tip(t) != l.Base || pushed(fault.calls) != 0 || res.RepairOutcome == landingResumed || l.Intent(t)["status"] != "prepared" || l.Intent(t)["commit_sha"] != prepared {
		t.Errorf("lane %s pushes %d outcome %q intent %v: a roll-forward that cannot move the ref resumes nothing", l.Tip(t), pushed(fault.calls), res.RepairOutcome, l.Intent(t))
	}
}

func TestWitness_NamesWhatItCouldNotRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		arrange func(t *testing.T, l landingLane, opts *Options) *gitFault
		want    string
	}{
		{name: "the cycle identity", want: "the cycle identity is unreadable: ",
			arrange: func(t *testing.T, l landingLane, opts *Options) *gitFault {
				opts.CycleID = 0
				mustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), "{")
				return realGit(func([]string) bool { return false }, 0)
			}},
		{name: "the run identity", want: "the run identity is unreadable: ",
			arrange: func(t *testing.T, l landingLane, opts *Options) *gitFault {
				opts.RunID = ""
				mustWrite(t, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), "{")
				return realGit(func([]string) bool { return false }, 0)
			}},
		{name: "the lane tip", want: "the lane tip is unreadable: ",
			arrange: func(t *testing.T, l landingLane, opts *Options) *gitFault {
				return realGit(argvIs("rev-parse", "HEAD"), 128)
			}},
		{name: "the newest audit of the run", want: "the newest audit is unreadable: ",
			arrange: func(t *testing.T, l landingLane, opts *Options) *gitFault {
				opts.RunID = "run-nobody-audited"
				return realGit(func([]string) bool { return false }, 0)
			}},
		{name: "the sealed explanation", want: "the sealed Build explanation is unreadable: ",
			arrange: func(t *testing.T, l landingLane, opts *Options) *gitFault {
				mustWrite(t, filepath.Join(l.Repo, ".evolve", "build-explanation-contracts", "cycle-1830.json"), "{")
				return realGit(func([]string) bool { return false }, 0)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := laneReadyToLand(t)
			in := l.preparedIntent(t)
			s, opts, _ := l.worktreeShip(nil)
			opts.Runner = tc.arrange(t, l, opts).runner()

			_, audit, why := s.witness(in)

			if !strings.HasPrefix(why, tc.want) || len(why) == len(tc.want) || audit != nil {
				t.Errorf("why = %q audit = %v, want a reason starting %q with its cause and no audit", why, audit, tc.want)
			}
		})
	}
}

func TestWitness_AReadableLaneWitnessesThePreparedIntent(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	in := l.preparedIntent(t)
	s, _, _ := l.worktreeShip(execRunner)

	w, audit, why := s.witness(in)

	if why != "" || audit == nil {
		t.Fatalf("why = %q audit = %v, want a full witness", why, audit)
	}
	want := landing.Witness{Cycle: in.Cycle, RunID: in.RunID, LaneTip: in.CommitSHA, Journaled: true, AuditArtifactSHA256: in.AuditArtifactSHA256,
		AuditedTree: in.AuditedTree, AuditPassed: true, ExplanationViewSHA256: in.ExplanationViewSHA256}
	if !reflect.DeepEqual(w, want) {
		t.Errorf("witness = %+v, want %+v", w, want)
	}
}

func TestExplanationViewSHA_HashesTheSealedViewDeterministically(t *testing.T) {
	l := laneReadyToLand(t)
	view, err := explanationdocs.LoadSnapshot(explanationdocs.CycleBinding{ProjectRoot: l.Repo, Cycle: landingCycle, RunID: landingRunID})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)

	first, ferr := explanationViewSHA(l.Repo, landingCycle, landingRunID)
	second, serr := explanationViewSHA(l.Repo, landingCycle, landingRunID)

	if ferr != nil || serr != nil || first != hex.EncodeToString(sum[:]) || first != second {
		t.Errorf("hashes %q, %q (errs %v, %v), want both the sha256 of the sealed view %x", first, second, ferr, serr, sum)
	}
	stubMarshalJSON(t, errors.New("encode refused"))
	if got, err := explanationViewSHA(l.Repo, landingCycle, landingRunID); err == nil || got != "" {
		t.Errorf("an unencodable view = %q, %v; want an error and no hash", got, err)
	}
}

func consumedLane(t *testing.T) (repo, parent, commit string, paths []string) {
	t.Helper()
	repo = makeRepo(t)
	src, dst := consumptionPair("2026-10-11T00-00-00Z-item.json")
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(src)), `{"id":"item"}`+"\n")
	runGit(t, repo, "add", "-f", src)
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "an inbox item")
	parent = headSHA(t, repo)
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(dst)), `{"id":"item","consumed":{"via":"ship"}}`+"\n")
	mustWrite(t, filepath.Join(repo, "work.txt"), "the lane's own change\n")
	runGit(t, repo, "rm", "-q", src)
	runGit(t, repo, "add", "-f", dst, "work.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "evolve-cycle 1830")
	return repo, parent, headSHA(t, repo), []string{src, dst}
}

func TestLaneTreeBeforeConsumption_RestoresEveryConsumedPathToItsParent(t *testing.T) {
	t.Parallel()
	repo, parent, commit, paths := consumedLane(t)
	opts := &Options{ProjectRoot: repo, Runner: execRunner, internalConsumedPaths: paths}

	tree, err := laneTreeBeforeConsumption(context.Background(), opts, commit)
	if err != nil {
		t.Fatal(err)
	}

	listing := runGitOut(t, repo, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(listing, paths[0]) || strings.Contains(listing, paths[1]) || !strings.Contains(listing, "work.txt") {
		t.Errorf("tree %s lists %q: the source returns, the consumed record goes, the lane's own change stays", tree, listing)
	}
	if diff := strings.TrimSpace(runGitOut(t, repo, "diff", "--name-status", parent, tree)); diff != "A\twork.txt" {
		t.Errorf("parent..measured = %q, want only the lane's own change: every consumed path is back at its parent shape", diff)
	}
	again, err := laneTreeBeforeConsumption(context.Background(), opts, commit)
	if err != nil || again != tree {
		t.Errorf("a second measure = %q, %v; want the same tree %s: the scratch index never touches the real one", again, err, tree)
	}
	if staged := runGitOut(t, repo, "diff", "--cached", "--name-only", "HEAD"); strings.TrimSpace(staged) != "" {
		t.Errorf("the real index changed to %q", staged)
	}
}

func TestLaneTreeBeforeConsumption_WithNoConsumptionIsTheCommitTree(t *testing.T) {
	t.Parallel()
	repo, _, commit, _ := consumedLane(t)

	tree, err := laneTreeBeforeConsumption(context.Background(), &Options{ProjectRoot: repo, Runner: execRunner}, commit)

	if want := strings.TrimSpace(runGitOut(t, repo, "rev-parse", commit+"^{tree}")); err != nil || tree != want {
		t.Errorf("tree = %q, %v; want the commit tree %s", tree, err, want)
	}
}

func TestLaneTreeBeforeConsumption_FailsLoudlyOnEveryScratchStep(t *testing.T) {
	cases := []struct {
		name    string
		fault   func(paths []string) *gitFault
		tmpdir  bool
		wantErr string
	}{
		{name: "no scratch dir", tmpdir: true, wantErr: "missing",
			fault: func([]string) *gitFault { return realGit(func([]string) bool { return false }, 0) }},
		{name: "the parent entry is unreadable", wantErr: "git [ls-tree",
			fault: func(paths []string) *gitFault { return realGit(argvStarts("ls-tree"), 128) }},
		{name: "the scratch index writes no tree", wantErr: "git write-tree in a scratch index wrote no tree",
			fault: func(paths []string) *gitFault { return realGit(argvIs("write-tree"), 0) }},
		{name: "the scratch index cannot remove an absent path", wantErr: "git [update-index --force-remove",
			fault: func(paths []string) *gitFault { return realGit(argvStarts("update-index", "--force-remove"), 1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, commit, paths := consumedLane(t)
			fault := tc.fault(paths)
			if tc.tmpdir {
				t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
			}

			tree, err := laneTreeBeforeConsumption(context.Background(), &Options{ProjectRoot: repo, Runner: fault.runner(), internalConsumedPaths: paths}, commit)

			if err == nil || tree != "" || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("tree = %q err = %v, want no tree and an error naming %q", tree, err, tc.wantErr)
			}
		})
	}
}

func TestRun_APostShipErrorAfterAResumeWarnsAndStillLands(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	l.failEveryPush(t, githubServerError)
	landingCommit := l.Tip(t)
	mustWrite(t, filepath.Join(l.Repo, ".evolve", "state.json"), "{")

	res, err := runShip(t, l.Repo, l.options(execRunner, &sleepLog{}))

	if err != nil || res.ExitCode != ExitOK || res.RepairOutcome != landingResumed || remoteHeadSHA(t, l.Repo) != landingCommit {
		t.Fatalf("exit=%d err=%v outcome=%q: the resumed landing is on origin, so a post-ship hook error never fails it", res.ExitCode, err, res.RepairOutcome)
	}
	if !containsLog(res, "[ship] WARN: post-ship hook error: ") {
		t.Errorf("logs %q lack the post-ship WARN", res.Logs)
	}
}

func TestRepairResumeUnpushed_ALandingThatCannotSettleIsNotARepair(t *testing.T) {
	t.Parallel()
	repo := mergedUnpushedFixture(t, "cycle-246-unsettled")
	head, audited := headSHA(t, repo), strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^"))
	headTree := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
	pushes := 0
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		sub := gitSubArgs(args)
		if name == "git" && len(sub) > 0 && sub[0] == "push" {
			pushes++
		}
		if name == "git" && pushes > 0 && reflect.DeepEqual(sub, []string{"rev-parse", head + "^{tree}"}) {
			_, _ = io.WriteString(stdout, strings.Repeat("f", 40)+"\n")
			return 0, nil
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	opts := &Options{Class: ClassCycle, ProjectRoot: repo, PluginRoot: repo, Runner: runner, Stdout: io.Discard, Stderr: io.Discard,
		internalAuditBoundTreeSHA: headTree}
	res := &RunResult{}

	outcome := repairResumeUnpushed(context.Background(), opts, res, &core.ShipError{Debug: map[string]string{"audited": audited}})

	if outcome != repairNone || res.RepairOutcome == "resume-pushed" {
		t.Errorf("outcome=%v repair_outcome=%q: a landing that cannot settle is not a completed repair", outcome, res.RepairOutcome)
	}
	if !containsLog(*res, "[ship] WARN: resume-unpushed landed "+head+" but cannot settle it: ") || remoteHeadSHA(t, repo) != head || pushes != 1 {
		t.Errorf("logs %q origin %s pushes %d: the WARN says the commit landed and why it did not settle", res.Logs, remoteHeadSHA(t, repo), pushes)
	}
}

func TestStrandedLanding_ReadsTheHeadItWouldResume(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fault   func() *gitFault
		state   string
		cycleID int
		want    bool
	}{
		{name: "an admissible stranded head", fault: func() *gitFault { return realGit(func([]string) bool { return false }, 0) }, want: true},
		{name: "an unreadable head", fault: func() *gitFault { return realGit(argvIs("rev-parse", "HEAD"), 128) }},
		{name: "an unreadable head tree", fault: func() *gitFault { return realGit(argvIs("rev-parse", "HEAD^{tree}"), 128) }},
		{name: "a detached head", fault: func() *gitFault { return realGit(argvIs("symbolic-ref", "--short", "HEAD"), 1) }},
		{name: "an unreadable branch", fault: func() *gitFault {
			f := realGit(argvIs("symbolic-ref", "--short", "HEAD"), 0)
			f.err = errors.New("git vanished")
			return f
		}},
		{name: "no cycle identity", state: `{"run_id":"test-run"}`, fault: func() *gitFault { return realGit(func([]string) bool { return false }, 0) }},
		{name: "an unreadable cycle state", state: "{", fault: func() *gitFault { return realGit(func([]string) bool { return false }, 0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := mergedUnpushedFixture(t, "cycle-246-stranded")
			if tc.state != "" {
				mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), tc.state)
			}
			head, audited := headSHA(t, repo), strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^"))
			headTree := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
			opts := &Options{Class: ClassCycle, ProjectRoot: repo, RunID: "test-run", Runner: tc.fault().runner(),
				internalAuditBoundTreeSHA: headTree, internalAuditArtifactSHA: "artifact"}

			in, ok := strandedLanding(context.Background(), opts, &core.ShipError{Debug: map[string]string{"audited": audited}})

			if ok != tc.want {
				t.Fatalf("stranded = %v, want %v (intent %+v)", ok, tc.want, in)
			}
			want := landing.Intent{}
			if tc.want {
				want = landing.Intent{Cycle: 1, RunID: "test-run", AuditArtifactSHA256: "artifact", AuditedTree: headTree, WorktreeBaseSHA: audited,
					CommitSHA: head, CommitTree: headTree, PreMain: head, Branch: "main", Status: landing.IntentPrepared}
			}
			if !reflect.DeepEqual(in, want) {
				t.Errorf("intent = %+v, want %+v", in, want)
			}
		})
	}
}

func directShipRepo(t *testing.T) string {
	t.Helper()
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	mustWrite(t, filepath.Join(repo, "change.txt"), "a change to ship\n")
	return repo
}

func directOpts(repo string, class Class, runner CmdRunner) *Options {
	return &Options{Class: class, CommitMessage: "evolve-cycle 1", ProjectRoot: repo, PluginRoot: repo, ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner: runner, Stdout: io.Discard, Stderr: io.Discard}
}

func TestShipDirect_EveryRefusalLeavesMainAndOriginUnmoved(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		class    Class
		arrange  func(t *testing.T, repo string, opts *Options)
		fault    func() *gitFault
		wantCode core.ShipErrorCode
		wantMsg  string
	}{
		{name: "the integrator lock is held", class: ClassManual, wantCode: core.CodeGitIO, wantMsg: "acquire integrator lock (.evolve/ship.lock): held",
			arrange: func(t *testing.T, repo string, opts *Options) {
				opts.shipLock = func(string) (func(), error) { return nil, errors.New("held") }
			}},
		{name: "the release set cannot be staged", class: ClassRelease, wantCode: core.CodeGitStageFailed, wantMsg: "git add (release set) failed (rc=1)",
			arrange: func(t *testing.T, repo string, opts *Options) {
				mustWrite(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n")
			},
			fault: func() *gitFault { return realGit(argvStarts("add", "--", "CHANGELOG.md"), 1) }},
		{name: "a large staged executable", class: ClassManual, wantCode: core.CodeGitStageFailed, wantMsg: "refusing to commit staged executable >1MB",
			arrange: func(t *testing.T, repo string, opts *Options) {
				path := filepath.Join(repo, "tools", "built")
				mustWrite(t, path, strings.Repeat("x", binaryStagingMaxBytes+1))
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "the staged tree is unreadable", class: ClassManual, wantCode: core.CodeGitIO, wantMsg: "exited 128",
			arrange: func(t *testing.T, repo string, opts *Options) {
				opts.internalAuditBoundTreeSHA = strings.Repeat("a", 40)
			},
			fault: func() *gitFault { return realGit(argvIs("write-tree"), 128) }},
		{name: "the staged tree is empty", class: ClassManual, wantCode: core.CodeGitIO, wantMsg: "git write-tree produced no tree SHA",
			arrange: func(t *testing.T, repo string, opts *Options) {
				opts.internalAuditBoundTreeSHA = strings.Repeat("a", 40)
			},
			fault: func() *gitFault { return realGit(argvIs("write-tree"), 0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := directShipRepo(t)
			before := headSHA(t, repo)
			fault := realGit(func([]string) bool { return false }, 0)
			if tc.fault != nil {
				fault = tc.fault()
			}
			opts := directOpts(repo, tc.class, fault.runner())
			tc.arrange(t, repo, opts)

			err := shipDirect(context.Background(), opts, &RunResult{}, "main")

			se := mustShipErr(t, err)
			if se.Code != tc.wantCode || !strings.Contains(se.Message, tc.wantMsg) {
				t.Errorf("err = %v, want %s containing %q", err, tc.wantCode, tc.wantMsg)
			}
			if headSHA(t, repo) != before || remoteHeadSHA(t, repo) != before || pushed(fault.calls) != 0 {
				t.Errorf("main %s origin %s pushes %d: a refusal before the commit moves nothing (want %s)", headSHA(t, repo), remoteHeadSHA(t, repo), pushed(fault.calls), before)
			}
		})
	}
}

func TestShipDirect_ACommittedTreeThatDriftedAfterThePushIsAnIntegrityBreach(t *testing.T) {
	t.Parallel()
	repo := directShipRepo(t)
	runGit(t, repo, "add", "change.txt")
	staged := strings.TrimSpace(runGitOut(t, repo, "write-tree"))
	pushes := 0
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		sub := gitSubArgs(args)
		if name == "git" && len(sub) > 0 && sub[0] == "push" {
			pushes++
		}
		if name == "git" && pushes > 0 && reflect.DeepEqual(sub, []string{"rev-parse", "HEAD^{tree}"}) {
			_, _ = io.WriteString(stdout, strings.Repeat("f", 40)+"\n")
			return 0, nil
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	opts := directOpts(repo, ClassManual, runner)
	opts.internalAuditBoundTreeSHA = staged
	res := &RunResult{}

	err := shipDirect(context.Background(), opts, res, "main")

	se := wantShipErr(t, err, core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, "main-path tree drift detected")
	if se.Stage != core.StagePostShip || se.Debug["committed_tree"] != strings.Repeat("f", 40) || se.Debug["phase"] != "post-push" {
		t.Errorf("stage %s debug %v: the breach names the post-push committed tree", se.Stage, se.Debug)
	}
	if !containsLog(*res, "pre-commit tree-SHA binding verified") || remoteHeadSHA(t, repo) != headSHA(t, repo) || pushes != 1 {
		t.Errorf("logs %q pushes %d: the pre-commit check passed and the commit is on origin, so only the post-push check catches the drift", res.Logs, pushes)
	}
}

func TestRun_AHeldIntegratorLockStopsAWorktreeShipBeforeItStages(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	opts := l.options(execRunner, &sleepLog{})
	opts.shipLock = func(string) (func(), error) { return nil, errors.New("held by a peer") }
	staged := strings.TrimSpace(runGitOut(t, l.WT, "write-tree"))

	res, err := runShip(t, l.Repo, opts)

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "acquire integrator lock (.evolve/ship.lock): held by a peer")
	if res.ExitCode == ExitOK || l.Tip(t) != l.Base || strings.TrimSpace(runGitOut(t, l.WT, "write-tree")) != staged || len(readShipJournal(l.Repo)) != 0 {
		t.Errorf("exit=%d lane=%s: a held lock commits, stages and journals nothing", res.ExitCode, l.Tip(t))
	}
}

func TestPrepareChanges_AnUnreadableLaneStatusStopsTheStage(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	fault := realGit(argvIs("status", "--porcelain", "-uall"), 128)
	s, _, _ := l.worktreeShip(fault.runner())

	changes, err := s.prepareChanges()

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "exited 128")
	if changes != worktreeNoChanges || fault.hits != 1 || l.Tip(t) != l.Base {
		t.Errorf("changes=%v status reads=%d lane=%s: an unreadable status reports no changes and commits nothing", changes, fault.hits, l.Tip(t))
	}
}
