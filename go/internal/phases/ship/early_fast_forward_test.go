package ship

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
)

type earlyFixture struct {
	repo, wt, base string
	packs          int
	onPack         func()
}

func newEarlyFixture(t *testing.T, stageChange bool) *earlyFixture {
	t.Helper()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-early")
	l := &earlyFixture{repo: repo, wt: wt}
	if stageChange {
		mustWrite(t, filepath.Join(wt, "lane.txt"), "lane change\n")
		runGit(t, wt, "add", "-A")
	}
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	return l
}

func (l *earlyFixture) seed(t *testing.T) {
	t.Helper()
	seedAudit(t, l.repo, "PASS")
	l.base = strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "HEAD"))
}

func (l *earlyFixture) moveMain(t *testing.T) {
	t.Helper()
	mustWrite(t, filepath.Join(l.repo, "peer.txt"), "a peer landed\n")
	runGit(t, l.repo, "add", "peer.txt")
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "a peer lands on main")
}

func (l *earlyFixture) pack(context.Context, string, string, string, string, io.Writer, func([]string)) error {
	l.packs++
	if l.onPack != nil {
		l.onPack()
	}
	return nil
}

func (l *earlyFixture) ship(t *testing.T, fleet bool) (core.PhaseResponse, error) {
	t.Helper()
	env := map[string]string{"EVOLVE_PLUGIN_ROOT": l.repo}
	if fleet {
		env[ipcenv.FleetKey] = "1"
	}
	p := New(Config{Runner: execRunner, RepoContractGate: "enforce", RepoContract: l.pack})
	return p.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: l.repo, Workspace: filepath.Join(l.repo, ".evolve", "runs", "cycle-1"),
		Worktree: l.wt, WorktreeBaseSHA: l.base, Env: env, Context: map[string]string{"commit_message": "evolve-cycle 1: goal=test"},
	})
}

func requireFleetRebase(t *testing.T, resp core.PhaseResponse, err error) {
	t.Helper()
	se, ok := core.AsShipError(err)
	if !ok || se.Code != core.CodeGitFleetRebaseNeeded || se.Class != core.ShipClassTransient || se.Stage != core.StageAtomicShip {
		t.Fatalf("err = %v, want GIT_FLEET_REBASE_NEEDED / transient / atomic-ship", err)
	}
	if resp.Signals["ship.error_code"] != string(se.Code) || resp.Signals["ship.error_class"] != string(se.Class) || resp.Signals["ship.error_stage"] != string(se.Stage) {
		t.Fatalf("signals = %v, want the ship error code, class and stage", resp.Signals)
	}
}

func TestPhaseRun_AMovedMainFailsFastBeforeTheRepoContractPack(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)
	l.moveMain(t)

	resp, err := l.ship(t, true)

	requireFleetRebase(t, resp, err)
	if l.packs != 0 {
		t.Fatalf("the repo-contract pack ran %d time(s) on a base that cannot fast-forward", l.packs)
	}
	if status := runGitOut(t, l.wt, "status", "--porcelain"); !strings.Contains(status, "lane.txt") {
		t.Fatalf("status = %q, want the audited change left pending in the lane", status)
	}
}

func TestPhaseRun_AMainThatDidNotMoveRunsThePackThenLands(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)

	resp, err := l.ship(t, true)

	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict=%s err=%v diagnostics=%v", resp.Verdict, err, resp.Diagnostics)
	}
	if l.packs != 1 {
		t.Fatalf("pack ran %d time(s), want 1", l.packs)
	}
	if got := strings.TrimSpace(runGitOut(t, l.repo, "ls-tree", "-r", "--name-only", "main")); !strings.Contains(got, "lane.txt") {
		t.Fatalf("main tree = %q, want the landed lane", got)
	}
}

func (l *earlyFixture) request() core.PhaseRequest {
	return core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: l.repo, Workspace: filepath.Join(l.repo, ".evolve", "runs", "cycle-1"),
		Worktree: l.wt, WorktreeBaseSHA: l.base, Env: map[string]string{"EVOLVE_PLUGIN_ROOT": l.repo, ipcenv.FleetKey: "1"},
	}
}

func TestEarlyFastForward_TheEarlyAndTheLandTimeErrorsAreOneError(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)
	l.moveMain(t)
	opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
	s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)
	if err := s.resolveCycleBranch(); err != nil {
		t.Fatal(err)
	}
	commit, err := s.landingCommit("")
	if err != nil {
		t.Fatal(err)
	}

	early := mustShipErr(t, checkEarlyFastForward(context.Background(), &opts))
	landTime := mustShipErr(t, s.checkLanding(commit))

	if early.Code != core.CodeGitFleetRebaseNeeded || early.Code != landTime.Code || early.Class != landTime.Class || early.Stage != landTime.Stage || early.Message != landTime.Message {
		t.Fatalf("early = %+v, land time = %+v, want one code, class, stage and message", early, landTime)
	}
}

func TestPhaseRun_ANonFleetShipKeepsTheLandTimeCheck(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)
	l.moveMain(t)

	_, err := l.ship(t, false)

	if l.packs != 1 {
		t.Fatalf("pack ran %d time(s), want 1: a non-fleet ship keeps today's order (err=%v)", l.packs, err)
	}
}

func TestPhaseRun_ALaneWithNoChangesSkipsTheEarlyCheck(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, false)
	l.seed(t)
	l.moveMain(t)

	_, err := l.ship(t, true)

	if l.packs != 1 {
		t.Fatalf("pack ran %d time(s), want 1: a lane that lands nothing must not fail early (err=%v)", l.packs, err)
	}
}

func TestPhaseRun_ACarriedLanePassesTheEarlyCheckAndRunsThePack(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.moveMain(t)
	runGit(t, l.wt, "merge", "--ff-only", "main")
	l.seed(t)

	resp, err := l.ship(t, true)

	if err != nil || resp.Verdict != core.VerdictPASS || l.packs != 1 {
		t.Fatalf("verdict=%s err=%v packs=%d, want a landed lane after one pack", resp.Verdict, err, l.packs)
	}
}

func TestPhaseRun_ALaneThatTookAnIdentityCarryKeepsTodaysOrder(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)
	l.moveMain(t)
	l.base = strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "HEAD"))

	_, err := l.ship(t, true)

	if l.packs != 1 {
		t.Fatalf("pack ran %d time(s), want 1: the auditor row names an older base than the lane (err=%v)", l.packs, err)
	}
}

func TestEarlyFastForwardApplies_PinsEachShipMode(t *testing.T) {
	t.Parallel()
	fleet := map[string]string{ipcenv.FleetKey: "1"}
	enforce := &Integration{Stage: "enforce"}
	for name, tc := range map[string]struct {
		opts Options
		want bool
	}{
		"fleet cycle ship":         {Options{Class: ClassCycle, Env: fleet}, true},
		"fleet shadow integration": {Options{Class: ClassCycle, Env: fleet, Integration: &Integration{Stage: "shadow"}}, true},
		"non-fleet cycle ship":     {Options{Class: ClassCycle, Env: map[string]string{ipcenv.FleetKey: "0"}}, false},
		"dry run":                  {Options{Class: ClassCycle, Env: fleet, DryRun: true}, false},
		"push only":                {Options{Class: ClassCycle, Env: fleet, PushOnly: true}, false},
		"deliver at enforce":       {Options{Class: ClassCycle, Env: fleet, Integration: enforce}, false},
		"manual class":             {Options{Class: ClassManual, Env: fleet}, false},
	} {
		if got := earlyFastForwardApplies(&tc.opts); got != tc.want {
			t.Errorf("%s: earlyFastForwardApplies = %v, want %v", name, got, tc.want)
		}
	}
}

func TestWorktreeShipCheckLanding_AlwaysRunsTheFastForwardCheckForANonFleetShip(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	l.seed(t)
	l.moveMain(t)
	req := l.request()
	delete(req.Env, ipcenv.FleetKey)
	opts := New(Config{Runner: execRunner}).shipOptions(req, "evolve-cycle 1")
	s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)
	if err := s.resolveCycleBranch(); err != nil {
		t.Fatal(err)
	}
	commit, err := s.landingCommit("")
	if err != nil {
		t.Fatal(err)
	}

	if se := mustShipErr(t, s.checkLanding(commit)); se.Code != core.CodeGitFFMergeDiverged {
		t.Fatalf("code = %s, want %s at land time for a non-fleet ship", se.Code, core.CodeGitFFMergeDiverged)
	}
}

func TestCheckEarlyFastForward_SkipsWhatItCannotJudge(t *testing.T) {
	t.Parallel()
	detach := func(dir string) func(*testing.T, *earlyFixture, *Options) {
		return func(t *testing.T, l *earlyFixture, _ *Options) {
			runGit(t, map[string]string{"repo": l.repo, "wt": l.wt}[dir], "checkout", "-q", "--detach")
		}
	}
	for name, tc := range map[string]struct {
		change func(*testing.T, *earlyFixture, *Options)
	}{
		"a worktree that is gone": {func(_ *testing.T, l *earlyFixture, o *Options) { o.ActiveWorktree = l.repo + "/gone" }},
		"the in-place project":    {func(_ *testing.T, l *earlyFixture, o *Options) { o.ActiveWorktree = l.repo }},
		"a detached project head": {detach("repo")},
		"a detached lane head":    {detach("wt")},
		"an unreadable lane head": {func(_ *testing.T, _ *earlyFixture, o *Options) {
			o.Runner = breakGitSubcommand("rev-parse")
		}},
		"no run id":            {func(_ *testing.T, _ *earlyFixture, o *Options) { o.RunID, o.WorkspacePath = "", "" }},
		"no audit for the run": {func(_ *testing.T, _ *earlyFixture, o *Options) { o.RunID = "another-run" }},
		"an unreadable status": {func(_ *testing.T, _ *earlyFixture, o *Options) {
			o.Runner = breakGitSubcommand("status")
		}},
	} {
		t.Run(name, func(t *testing.T) {
			l := newEarlyFixture(t, true)
			l.seed(t)
			l.moveMain(t)
			opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
			if name == "no run id" {
				mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), "{not json")
			}
			tc.change(t, l, &opts)

			if err := checkEarlyFastForward(context.Background(), &opts); err != nil {
				t.Fatalf("checkEarlyFastForward = %v, want a skip: the later stages report this state", err)
			}
		})
	}
}

func TestCarriesWork_ReadsBothTheDirtyLaneAndTheCommitsAheadOfMain(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		prepare func(*testing.T, *earlyFixture)
		fail    string
		want    bool
	}{
		"a dirty lane": {func(*testing.T, *earlyFixture) {}, "", true},
		"a committed lane": {func(t *testing.T, l *earlyFixture) {
			runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane")
		}, "", true},
		"a lane with nothing":        {func(t *testing.T, l *earlyFixture) { runGit(t, l.wt, "reset", "-q", "--hard") }, "", false},
		"an unreadable commit count": {func(t *testing.T, l *earlyFixture) { runGit(t, l.wt, "reset", "-q", "--hard") }, "rev-list", false},
		"an unreadable tracked diff": {func(t *testing.T, l *earlyFixture) { runGit(t, l.wt, "reset", "-q", "--hard") }, "diff", false},
		"only untracked files the manifest does not select": {func(t *testing.T, l *earlyFixture) {
			runGit(t, l.wt, "reset", "-q", "--hard")
			mustWrite(t, filepath.Join(l.wt, "scratch.txt"), "not declared\n")
		}, "", false},
		"a tracked edit": {func(t *testing.T, l *earlyFixture) {
			runGit(t, l.wt, "reset", "-q", "--hard")
			mustWrite(t, filepath.Join(l.wt, "fixture.txt"), "edited\n")
		}, "", true},
	} {
		t.Run(name, func(t *testing.T) {
			l := newEarlyFixture(t, true)
			tc.prepare(t, l)
			opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
			if tc.fail != "" {
				opts.Runner = breakGitSubcommand(tc.fail)
			}
			s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)

			if got := s.carriesWork(); got != tc.want {
				t.Fatalf("carriesWork = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCarriesWork_AnUntrackedFileTheSelectionStagesCountsAsWork(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, false)
	mustWrite(t, filepath.Join(l.wt, "staged-by-selection.txt"), "new\n")
	opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
	opts.WorkspacePath = ""
	s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)

	if !s.carriesWork() {
		t.Fatal("carriesWork = false, want true: the explicit-path selection stages this file")
	}
}

func TestCheckEarlyFastForward_ACommittedLaneOnTheTipPasses(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane")
	l.seed(t)
	opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")

	if err := checkEarlyFastForward(context.Background(), &opts); err != nil {
		t.Fatalf("checkEarlyFastForward = %v, want nil: main did not move", err)
	}
}

type catchUpFixture struct {
	*earlyFixture
	bare, peer string
}

func newCatchUpFixture(t *testing.T) *catchUpFixture {
	t.Helper()
	l := newEarlyFixture(t, true)
	bare := strings.TrimSpace(runGitOut(t, l.repo, "remote", "get-url", "origin"))
	l.seed(t)
	runGit(t, l.repo, "push", "-q", "origin", "main")
	peer := pushOnTopOf(t, bare, l.base)
	runGit(t, l.repo, "fetch", "-q", "origin")
	return &catchUpFixture{earlyFixture: l, bare: bare, peer: peer}
}

func (c *catchUpFixture) lane(t *testing.T) *worktreeShip {
	t.Helper()
	opts := New(Config{Runner: execRunner}).shipOptions(c.request(), "evolve-cycle 1")
	return newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", c.wt)
}

func TestPlanCatchUp_NamesTheOriginWithoutMovingMain(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		journaled bool
		advances  bool
	}{
		"origin ahead and journaled":  {true, true},
		"origin ahead, not journaled": {false, false},
	} {
		t.Run(name, func(t *testing.T) {
			c := newCatchUpFixture(t)
			s := c.lane(t)
			if tc.journaled {
				if err := appendShipJournal(c.repo, shipjournal.Entry{SHA: c.peer}); err != nil {
					t.Fatal(err)
				}
			}

			plan := s.planCatchUp()

			if plan.advances() != tc.advances || plan.origin != c.peer {
				t.Fatalf("plan = %+v, want advances=%v origin=%q", plan, tc.advances, c.peer)
			}
			if main := strings.TrimSpace(runGitOut(t, c.repo, "rev-parse", "main")); main != c.base {
				t.Fatalf("main = %s, want %s: the plan is a query and moves nothing", main, c.base)
			}
		})
	}
}

func TestPlanCatchUp_HasNothingToDoWithoutAnAheadAncestorOrigin(t *testing.T) {
	t.Parallel()
	t.Run("no origin ref", func(t *testing.T) {
		l := newEarlyFixture(t, true)
		l.seed(t)
		opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
		s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)
		if plan := s.planCatchUp(); plan.advances() || plan.origin != "" {
			t.Fatalf("plan = %+v, want an empty plan", plan)
		}
	})
	t.Run("origin equals main", func(t *testing.T) {
		l := newEarlyFixture(t, true)
		l.seed(t)
		runGit(t, l.repo, "push", "-q", "origin", "main")
		runGit(t, l.repo, "fetch", "-q", "origin")
		opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
		s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)
		if plan := s.planCatchUp(); plan.advances() {
			t.Fatalf("plan = %+v, want an empty plan", plan)
		}
	})
	t.Run("origin diverged from main", func(t *testing.T) {
		c := newCatchUpFixture(t)
		c.moveMain(t)
		s := c.lane(t)
		if plan := s.planCatchUp(); plan.advances() {
			t.Fatalf("plan = %+v, want an empty plan: ship never fast-forwards over a divergence", plan)
		}
	})
}

func TestCatchUpMain_MovesMainOnlyOverJournaledCommits(t *testing.T) {
	t.Parallel()
	t.Run("journaled origin: fast-forward", func(t *testing.T) {
		c := newCatchUpFixture(t)
		if err := appendShipJournal(c.repo, shipjournal.Entry{SHA: c.peer}); err != nil {
			t.Fatal(err)
		}
		s := c.lane(t)

		s.catchUpMain()

		if strings.TrimSpace(runGitOut(t, c.repo, "rev-parse", "main")) != c.peer {
			t.Fatalf("main did not fast-forward to %s", c.peer)
		}
		if !containsLog(*s.result, "fast-forwards main to "+c.peer) {
			t.Fatalf("logs = %q", s.result.Logs)
		}
	})
	t.Run("origin not journaled: no move", func(t *testing.T) {
		c := newCatchUpFixture(t)
		s := c.lane(t)

		s.catchUpMain()

		if strings.TrimSpace(runGitOut(t, c.repo, "rev-parse", "main")) != c.base {
			t.Fatalf("main moved, want it left on %s", c.base)
		}
		if !containsLog(*s.result, "the ship journal does not hold "+c.peer) {
			t.Fatalf("logs = %q", s.result.Logs)
		}
	})
}

func TestCheckEarlyFastForward_SkipsALaneWhoseMainIsBehindAJournaledOrigin(t *testing.T) {
	t.Parallel()
	c := newCatchUpFixture(t)
	if err := appendShipJournal(c.repo, shipjournal.Entry{SHA: c.peer}); err != nil {
		t.Fatal(err)
	}
	opts := New(Config{Runner: execRunner}).shipOptions(c.request(), "evolve-cycle 1")

	if err := checkEarlyFastForward(context.Background(), &opts); err != nil {
		t.Fatalf("checkEarlyFastForward = %v, want a skip: land time catches main up under the lock first", err)
	}
	if main := strings.TrimSpace(runGitOut(t, c.repo, "rev-parse", "main")); main != c.base {
		t.Fatalf("main = %s, want %s: the early check moves nothing", main, c.base)
	}
}

func TestLand_ACaughtUpMainIsTheTipTheLandTimeCheckJudges(t *testing.T) {
	t.Parallel()
	c := newCatchUpFixture(t)
	if err := appendShipJournal(c.repo, shipjournal.Entry{SHA: c.peer}); err != nil {
		t.Fatal(err)
	}
	s := c.lane(t)
	if err := s.resolveCycleBranch(); err != nil {
		t.Fatal(err)
	}
	commit, err := s.landingCommit("")
	if err != nil {
		t.Fatal(err)
	}

	s.catchUpMain()
	se := mustShipErr(t, s.checkLanding(commit))

	if main := strings.TrimSpace(runGitOut(t, c.repo, "rev-parse", "main")); main != c.peer {
		t.Fatalf("main = %s, want it caught up to %s", main, c.peer)
	}
	if se.Code != core.CodeGitFleetRebaseNeeded {
		t.Fatalf("code = %s, want %s: the lane lacks the caught-up main", se.Code, core.CodeGitFleetRebaseNeeded)
	}
}

func breakGitSubcommand(sub string) CmdRunner {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && slices.Contains(args, sub) {
			return 1, fmt.Errorf("git %s broke", sub)
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func TestNew_RepoContractFuncDefaultsToTheRealGateAndKeepsAnInjectedOne(t *testing.T) {
	t.Parallel()
	if New(Config{}).repoContract == nil {
		t.Fatal("New left the repo-contract entry empty, so the gate would panic")
	}
	called := false
	var injected RepoContractFunc = func(context.Context, string, string, string, string, io.Writer, func([]string)) error {
		called = true
		return nil
	}

	p := New(Config{RepoContract: injected})
	if err := p.repoContract(context.Background(), "", "", "", "", io.Discard, nil); err != nil || !called {
		t.Fatalf("injected entry called=%v err=%v, want it kept", called, err)
	}
}

func TestCarriesWork_OnlyBinaryChurnIsNoWork(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	runGit(t, l.wt, "reset", "-q", "--hard")
	mustWrite(t, filepath.Join(l.wt, "fixture.txt"), "a rebuilt binary\n")
	opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")
	opts.ShipBinaryPath = filepath.Join(opts.ProjectRoot, "fixture.txt")
	s := newWorktreeShip(context.Background(), &opts, &RunResult{}, "main", l.wt)

	if s.carriesWork() {
		t.Fatal("carriesWork = true, want false: prepareChanges discards binary churn, so the lane lands nothing")
	}
}

func TestCheckEarlyFastForward_SkipsWhenTheLocalMainMovedAndAJournaledOriginIsFurtherAhead(t *testing.T) {
	t.Parallel()
	l := newEarlyFixture(t, true)
	bare := strings.TrimSpace(runGitOut(t, l.repo, "remote", "get-url", "origin"))
	l.seed(t)
	l.moveMain(t)
	moved := strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "main"))
	runGit(t, l.repo, "push", "-q", "origin", "main")
	peer := pushOnTopOf(t, bare, moved)
	runGit(t, l.repo, "fetch", "-q", "origin")
	if err := appendShipJournal(l.repo, shipjournal.Entry{SHA: peer}); err != nil {
		t.Fatal(err)
	}
	opts := New(Config{Runner: execRunner}).shipOptions(l.request(), "evolve-cycle 1")

	if err := checkEarlyFastForward(context.Background(), &opts); err != nil {
		t.Fatalf("checkEarlyFastForward = %v, want a skip: land time catches main up to the journaled origin first", err)
	}
}
