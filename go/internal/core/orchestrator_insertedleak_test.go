//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// insertedLeakPlanJSON mints a single bug-reproduction phase that does not
// opt out of source writes, so it inherits the cycle worktree;
// ClampPlanToFloorWith re-adds the mandatory spine, as in production.
const insertedLeakPlanJSON = `[{"phase":"bug-reproduction","run":true,"justification":"reproduce the reported defect as a failing test before build hardening","mint":{"prompt":"You write a failing test that reproduces the reported bug.","tier":"balanced","cli":"claude"}}]`

const insertedLeakRelPath = "go/internal/looppreflight/bug_reproduction_test.go"

// leakMinter's Register lets each subtest choose where the minted phase
// writes (main tree vs worktree) via onRun.
type leakMinter struct {
	onRun func(req PhaseRequest)
}

func (m leakMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error) {
	spec := cfg.Spec()
	spec.Optional = true
	return spec, &insertedLeakRunner{name: spec.Name, onRun: m.onRun}, nil
}

// insertedLeakRunner lives in orchestrator_testfakes_test.go, shared with the
// fast-tier spinegate test.

// initInsertedLeakRepo commits one source file first, so the leaked path is a
// NEW untracked file — the shape the guard's untracked-only baseline must catch.
func initInsertedLeakRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	p := filepath.Join(root, "go", "internal", "looppreflight", "boot.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("package looppreflight\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return root
}

// realLeakWorktree provisions a REAL `git worktree add` outside the repo, so
// recoverBuildLeak's `git add -f` on the relocated leak succeeds.
type realLeakWorktree struct {
	t    *testing.T
	path string
}

func (w *realLeakWorktree) Create(projectRoot string, _ int) (string, error) {
	if w.path != "" {
		return w.path, nil // idempotent: reuse the cycle's worktree
	}
	wt := filepath.Join(w.t.TempDir(), "wt")
	gitInRepo(w.t, projectRoot, "worktree", "add", "--detach", "-q", wt, "HEAD")
	w.path = wt
	return wt, nil
}

// Cleanup is a deliberate no-op: a shipped cycle's worktree must survive for
// post-run `git diff HEAD` inspection; t.TempDir handles teardown.
func (w *realLeakWorktree) Cleanup(_, _ string) error { return nil }

func insertedLeakOrchestrator(t *testing.T, planJSON string, onRun func(PhaseRequest)) (*Orchestrator, *realLeakWorktree) {
	t.Helper()
	plan, err := parsePhasePlan(planJSON)
	if err != nil {
		t.Fatalf("parsePhasePlan: %v", err)
	}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build", "audit", "ship"}

	wt := &realLeakWorktree{t: t}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}),
		WithPlanner(&fixedPlanner{plan: plan}),
		WithRegistrar(leakMinter{onRun: onRun}),
		WithWorktreeProvisioner(wt))
	return o, wt
}

func TestInsertedPhaseMainTreeLeakRecovers(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initInsertedLeakRepo(t)
	// runner.Run executes synchronously on the test goroutine inside RunCycle,
	// so a plain bool is race-free; -race will surface it if that ever changes.
	mintRan := false
	o, wt := insertedLeakOrchestrator(t, insertedLeakPlanJSON, func(req PhaseRequest) {
		mintRan = true
		// Fatalf, not Errorf: a half-executed leak write must abort immediately,
		// or the test reports a spurious guard RED.
		p := filepath.Join(root, filepath.FromSlash(insertedLeakRelPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir leak dir: %v", err)
		}
		if err := os.WriteFile(p, []byte("package looppreflight\n// leaked\n"), 0o644); err != nil {
			t.Fatalf("leak write: %v", err)
		}
	})

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "g",
	})

	if !mintRan {
		t.Fatalf("precondition: minted phase never dispatched (phases=%v) — the replay did not reach the seam under test", res.PhasesRun)
	}
	if err != nil {
		t.Fatalf("leak-recovery must auto-heal the main-tree leak and let the cycle complete; got abort: %v", err)
	}
	if !slices.Contains(res.PhasesRun, PhaseShip) {
		t.Errorf("ship never ran — cycle did not complete after recovery (phases=%v)", res.PhasesRun)
	}
	// Checks the specific leaked path, not a blanket porcelain-empty: a full
	// RunCycle legitimately leaves .evolve/ residue in the main tree.
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(insertedLeakRelPath))); !os.IsNotExist(statErr) {
		t.Errorf("leaked file must be removed from the main tree; stat err=%v", statErr)
	}
	if st := gitInRepo(t, root, "status", "--porcelain", "-uall"); strings.Contains(st, insertedLeakRelPath) {
		t.Errorf("leaked path %s must not remain in the main tree porcelain; got:\n%s", insertedLeakRelPath, st)
	}
	if wt.path == "" {
		t.Fatal("worktree was never provisioned — recovery had nowhere to relocate the leak")
	}
	if _, err := os.Stat(filepath.Join(wt.path, filepath.FromSlash(insertedLeakRelPath))); err != nil {
		t.Errorf("leaked file must be relocated into the worktree: %v", err)
	}
	if diff := gitInRepo(t, wt.path, "diff", "HEAD", "--name-only"); !strings.Contains(diff, insertedLeakRelPath) {
		t.Errorf("relocated file must be staged/visible to `git diff HEAD` in the worktree; got %q", diff)
	}
}

func TestInsertedPhaseWorktreeWriteIsClean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	mintRan := false // synchronous runner.Run — see Test 1's race note
	o, _ := insertedLeakOrchestrator(t, insertedLeakPlanJSON, func(req PhaseRequest) {
		mintRan = true
		if req.Worktree == "" {
			t.Error("minted write-capable phase dispatched with Worktree=\"\" (cycle-280 regression)")
			return
		}
		p := filepath.Join(req.Worktree, filepath.FromSlash(insertedLeakRelPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir worktree dir: %v", err)
		}
		if err := os.WriteFile(p, []byte("package looppreflight\n// in worktree\n"), 0o644); err != nil {
			t.Fatalf("worktree write: %v", err)
		}
	})
	root := initInsertedLeakRepo(t)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "g",
	})
	if !mintRan {
		t.Fatalf("precondition: minted phase never dispatched (phases=%v) — the discriminator was not exercised", res.PhasesRun)
	}
	if err != nil {
		t.Fatalf("a worktree-confined write must not abort the cycle: %v", err)
	}
	if !slices.Contains(res.PhasesRun, PhaseShip) {
		t.Errorf("ship never ran — cycle did not complete (phases=%v)", res.PhasesRun)
	}
}
