package core_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

// initConflictRebaseRepoT is initCleanRebaseRepoT with one overlapping file: main and the cycle both rewrite
// shared.md, so the fleet rebase replays into a genuine conflict.
func initConflictRebaseRepoT(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		if err := writeFileT(filepath.Join(dir, rel), content); err != nil {
			t.Fatal(err)
		}
	}
	runGitT(t, dir, "init")
	runGitT(t, dir, "checkout", "-b", "main")
	runGitT(t, dir, "config", "user.email", "t@example.com")
	runGitT(t, dir, "config", "user.name", "test")
	runGitT(t, dir, "config", "commit.gpgsign", "false")
	write("base.txt", "base\n")
	write("shared.md", "base line\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "base")
	runGitT(t, dir, "checkout", "-b", "cycle")
	write("lane.txt", "lane change\n")
	write("shared.md", "lane line\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "lane change")
	runGitT(t, dir, "checkout", "main")
	write("shared.md", "peer line\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "peer change")
	runGitT(t, dir, "checkout", "cycle")
	return dir
}

// committingShip does what the real ship does before its fast-forward: it commits the worktree, then fails with
// RebaseNeeded whenever main is not an ancestor of HEAD.
type committingShip struct {
	calls    int
	landedOn []bool
}

func (s *committingShip) Name() string { return string(core.PhaseShip) }
func (s *committingShip) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.calls++
	if out, err := exec.Command("git", "-C", req.Worktree, "status", "--porcelain").Output(); err == nil && len(out) != 0 {
		_ = exec.Command("git", "-C", req.Worktree, "add", "-u").Run()
		_ = exec.Command("git", "-C", req.Worktree, "add", "--", "docs/explain").Run()
		_ = exec.Command("git", "-C", req.Worktree, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "ship commit").Run()
	}
	onMain := exec.Command("git", "-C", req.Worktree, "merge-base", "--is-ancestor", "main", "HEAD").Run() == nil
	s.landedOn = append(s.landedOn, onMain)
	if !onMain {
		return core.PhaseResponse{Phase: string(core.PhaseShip), Verdict: core.VerdictFAIL},
			core.NewShipError(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip, "peer moved main")
	}
	return core.PhaseResponse{Phase: string(core.PhaseShip), Verdict: core.VerdictPASS}, nil
}

// resolvingDebugger takes main's version of the conflicted file in the worktree, as 1719's debugger did, and
// asks for a reship.
type resolvingDebugger struct {
	calls   int
	writes  string
	signals map[string]any
}

func (d *resolvingDebugger) Name() string { return string(core.PhaseDebugger) }
func (d *resolvingDebugger) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	d.calls++
	if err := writeFileT(filepath.Join(req.Worktree, "shared.md"), d.writes); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := writeFileT(filepath.Join(req.Worktree, "docs", "debugger-scratch.md"), "untracked debris\n"); err != nil {
		return core.PhaseResponse{}, err
	}
	signals := d.signals
	if signals == nil {
		signals = map[string]any{"debugger.action": "RESHIP"}
	}
	return core.PhaseResponse{Phase: string(core.PhaseDebugger), Verdict: core.VerdictPASS, Signals: signals}, nil
}

type conflictFixture struct {
	dir   string
	ship  *committingShip
	audit *countingRunner
	build *explanationWritingRunner
	tdd   *countingRunner
	dbg   *resolvingDebugger
	o     *core.Orchestrator
}

func newConflictFixture(t *testing.T, debuggerWrites string, signals map[string]any) conflictFixture {
	t.Helper()
	f := conflictFixture{dir: initConflictRebaseRepoT(t), ship: &committingShip{}, audit: &countingRunner{name: "audit"}, build: &explanationWritingRunner{}, tdd: &countingRunner{name: "tdd"}, dbg: &resolvingDebugger{writes: debuggerWrites, signals: signals}}
	f.o = core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:     f.ship,
		core.PhaseAudit:    f.audit,
		core.PhaseBuild:    f.build,
		core.PhaseTDD:      f.tdd,
		core.PhaseDebugger: f.dbg,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: f.dir}))
	return f
}

func TestRecoverFromShipError_ADebuggersBuildRerunRunsOnTheRebasedTree(t *testing.T) {
	f := newConflictFixture(t, "peer line\n", map[string]any{"debugger.action": "RERUN_PHASE", "debugger.rerun_phase": "build"})
	if err := runCompositionCycle(t, f.o); err != nil {
		t.Fatalf("a conflict the debugger resolved must ship: %v", err)
	}
	if f.tdd.calls != 1 || f.build.calls != 2 || f.audit.calls != 2 || f.ship.calls != 2 || !f.ship.landedOn[1] {
		t.Fatalf("calls tdd=%d build=%d audit=%d ship=%d landedOn=%v: the debugger's Build re-run happens on the rebased tree, then Audit, then the reship on main", f.tdd.calls, f.build.calls, f.audit.calls, f.ship.calls, f.ship.landedOn)
	}
}

func TestRecoverFromShipError_ADebuggersTestsFirstRequestYieldsToTheRebasedRoute(t *testing.T) {
	f := newConflictFixture(t, "peer line\n", map[string]any{"debugger.action": "RERUN_PHASE", "debugger.rerun_phase": "tdd"})
	if err := runCompositionCycle(t, f.o); err != nil {
		t.Fatalf("a conflict the debugger resolved must ship: %v", err)
	}
	if f.tdd.calls != 1 || f.build.calls != 2 || f.audit.calls != 2 || f.ship.calls != 2 || !f.ship.landedOn[1] {
		t.Fatalf("calls tdd=%d build=%d audit=%d ship=%d landedOn=%v: after a rebase the route's Build re-authors; TDD does not re-run", f.tdd.calls, f.build.calls, f.audit.calls, f.ship.calls, f.ship.landedOn)
	}
}

func TestRecoverFromShipError_AConflictTheDebuggerLeftEndsTheCycleWithoutAReship(t *testing.T) {
	f := newConflictFixture(t, "lane line\n", nil)
	_ = runCompositionCycle(t, f.o)
	if f.dbg.calls != 1 || f.ship.calls != 1 {
		t.Fatalf("calls debugger=%d ship=%d, want 1/1: a conflict that survives the debugger is not reshipped on the stale base", f.dbg.calls, f.ship.calls)
	}
}

func TestRecoverFromShipError_ADebuggerResolvedConflictReentersTheRebaseNotAStaleReship(t *testing.T) {
	f := newConflictFixture(t, "peer line\n", nil)
	dir, ship, audit, build, dbg, o := f.dir, f.ship, f.audit, f.build, f.dbg, f.o

	if err := runCompositionCycle(t, o); err != nil {
		t.Fatalf("a conflict the debugger resolved must ship: %v", err)
	}
	if dbg.calls != 1 {
		t.Fatalf("debugger calls = %d, want 1", dbg.calls)
	}
	if ship.calls != 2 || len(ship.landedOn) != 2 || ship.landedOn[0] || !ship.landedOn[1] {
		t.Fatalf("ship calls = %d landedOn = %v, want 2 (diverged, then on main): the resolved tree must be rebased before any reship", ship.calls, ship.landedOn)
	}
	if build.calls != 2 || audit.calls != 2 {
		t.Fatalf("calls build=%d audit=%d, want 2/2: a resolution that changed bytes is re-authored and re-audited on the new base before it ships", build.calls, audit.calls)
	}
	if out := runGitT(t, dir, "show", "HEAD:shared.md"); out != "peer line\n" {
		t.Fatalf("the shipped tree carries the debugger's resolution: shared.md = %q", out)
	}
	if exec.Command("git", "-C", dir, "cat-file", "-e", "HEAD:docs/debugger-scratch.md").Run() == nil {
		t.Fatal("the debugger's untracked scratch file rode the carrier into the shipped tree")
	}
}

type fencedDebugger struct {
	inner    *resolvingDebugger
	writable []string
	kept     []string
}

func (d *fencedDebugger) Name() string { return string(core.PhaseDebugger) }
func (d *fencedDebugger) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	d.writable = req.WorktreeWritablePaths
	fence := treefence.Begin(ctx, req.Worktree, req.WorktreeReadOnly, req.WorktreeWritablePaths...)
	resp, err := d.inner.Run(ctx, req)
	d.kept = fence.End(ctx).Kept
	return resp, err
}

func TestRecoverFromShipError_TheFenceKeepsTheDebuggersResolutionOfTheConflictedFile(t *testing.T) {
	f := newConflictFixture(t, "peer line\n", nil)
	fenced := &fencedDebugger{inner: f.dbg}
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:     f.ship,
		core.PhaseAudit:    f.audit,
		core.PhaseBuild:    f.build,
		core.PhaseTDD:      f.tdd,
		core.PhaseDebugger: fenced,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: f.dir}))

	if err := runCompositionCycle(t, o); err != nil {
		t.Fatalf("a conflict the fenced debugger resolved must ship: %v", err)
	}
	if f.ship.calls != 2 || !f.ship.landedOn[1] {
		t.Fatalf("ship calls = %d landedOn = %v, want the second ship to land on main", f.ship.calls, f.ship.landedOn)
	}
	if !reflect.DeepEqual(fenced.writable, []string{"shared.md"}) || !reflect.DeepEqual(fenced.kept, []string{"shared.md"}) {
		t.Fatalf("debugger writable = %v kept = %v, want both [shared.md]", fenced.writable, fenced.kept)
	}
	if out := runGitT(t, f.dir, "show", "HEAD:shared.md"); out != "peer line\n" {
		t.Fatalf("the shipped tree must carry the resolution the fence kept: shared.md = %q", out)
	}
	if f.build.calls != 2 || f.audit.calls != 2 {
		t.Fatalf("calls build=%d audit=%d, want 2/2: the kept resolution changed bytes, so it is re-authored and re-audited before it ships", f.build.calls, f.audit.calls)
	}
}

func TestResumeFleetRebaseAfterDebugger_ASpentBudgetOrAnInPlaceWorktreeTouchesNoGit(t *testing.T) {
	dir := initConflictRebaseRepoT(t)
	before := runGitT(t, dir, "rev-parse", "HEAD")
	center := signalcenter.New()
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil), core.WithSignalCenter(center))
	cs := core.CycleState{RunID: "run-1", ShipRecoveryCode: string(core.CodeGitFleetRebaseNeeded), ActiveWorktree: dir}
	if next, resumed := o.ResumeFleetRebaseAfterDebuggerForTest(context.Background(), t.TempDir(), 1, &cs, core.PhaseShip, 5, 1); !resumed || next != core.PhaseEnd {
		t.Fatalf("a spent budget ends the cycle: next=%q resumed=%v", next, resumed)
	}
	if events := center.Recent(); len(events) != 1 || events[0].Code != core.CodeRebaseReentryAborted || events[0].Kind != signalcenter.KindPhaseAborted || !strings.Contains(events[0].Reason, "exhausted") {
		t.Fatalf("the abort is a Signal Center event naming its reason: %+v", events)
	}
	if next, resumed := o.ResumeFleetRebaseAfterDebuggerForTest(context.Background(), dir, 1, &cs, core.PhaseShip, 0, 1); resumed || next != "" {
		t.Fatalf("an in-place worktree is never rebased by a cycle: next=%q resumed=%v", next, resumed)
	}
	if after := runGitT(t, dir, "rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD moved from %s to %s without a re-entry", before, after)
	}
}

func TestEarliestPhase_KeepsAnUpstreamDebuggerTarget(t *testing.T) {
	cases := []struct{ decided, routed, want core.Phase }{
		{core.PhaseTDD, core.PhaseBuild, core.PhaseBuild},
		{core.PhaseBuild, core.PhaseAudit, core.PhaseBuild},
		{core.PhaseShip, core.PhaseBuild, core.PhaseBuild},
		{core.PhaseAudit, core.PhaseShip, core.PhaseAudit},
		{core.PhaseEnd, core.PhaseBuild, core.PhaseBuild},
	}
	for _, c := range cases {
		if got := core.EarliestPhaseForTest(c.decided, c.routed); got != c.want {
			t.Fatalf("earliest(%s, %s) = %s, want %s", c.decided, c.routed, got, c.want)
		}
	}
}

func TestFleetRebaseRecovery_NamesOnlyTheFleetRebaseCodes(t *testing.T) {
	for _, code := range []string{"", string(core.CodeWorktreeResolve), string(core.CodeAuditBindingHeadMoved)} {
		if core.FleetRebaseRecoveryForTest(code) {
			t.Fatalf("%q is not a fleet-rebase recovery", code)
		}
	}
	for _, code := range []string{string(core.CodeGitFleetRebaseNeeded), string(core.CodeGitFleetRebaseConflict)} {
		if !core.FleetRebaseRecoveryForTest(code) {
			t.Fatalf("%q is a fleet-rebase recovery", code)
		}
	}
}
