//go:build integration

// orchestrator_insertedleak_test.go — acceptance pin for inbox defect
// 2026-06-10T09-40-00Z-inserted-phase-treediff-guard-gap (cycle-270 replay).
//
// Cycle-270: the advisor-inserted bug-reproduction phase wrote
// go/internal/looppreflight/bug_reproduction_test.go into the MAIN tree and no
// tree-diff guard fired — the cycle proceeded to audit/ship as if clean. The
// guard then was snapshot-scoped to source-writing SPINE phases; inserted/
// minted phases were invisible to it.
//
// Production has since moved (7e0df0b5 one-classifier-for-all-phases +
// 02b778ef writes_source-keyed worktree dispatch): the pre-phase snapshot and
// post-phase check now run for EVERY dispatched phase. These tests pin the
// inbox item's acceptance at the genuine end-to-end seam — parsePhasePlan
// (real advisor-output parser) → ClampPlanToFloorWith → registerMintedPhases
// → dispatch loop → tree-diff guard — so the coverage can never silently
// regress to phase-identity scoping again:
//
//  1. A minted phase that writes a NEW source file into the MAIN tree (the
//     exact cycle-270 shape: untracked, so the porcelain — not diff-HEAD —
//     path must catch it) is AUTO-RECOVERED: leak-recovery (cycle-528/529)
//     relocates the leaked file into the phase's real worktree, stages it, and
//     restores the main tree clean, so the cycle completes and ships with the
//     work still visible to the auditor's `git diff HEAD`. This preserves
//     cycle-270's true guarantee — leaked work must never be invisible to audit
//     — via the newer non-aborting mechanism (the file lands in the worktree,
//     not silently in main). Recovery needs a REAL git worktree to `git add`
//     into: the earlier fake non-git t.TempDir() made that add fail, aborting
//     the cycle via the wrong path (cycle-535 CI-red root cause).
//  2. The same minted phase writing inside its provisioned worktree is clean:
//     the cycle continues to ship. The discriminator that keeps the guard
//     honest — isolation is the contract, not "minted phases always recover".
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

// insertedLeakPlanJSON replays the cycle-270 advisor output shape: a single
// minted bug-reproduction phase inserted after build, NOT opting out of source
// writes (so it inherits the cycle worktree per 02b778ef). The mandatory spine
// is re-added by ClampPlanToFloorWith, exactly as in production.
const insertedLeakPlanJSON = `[{"phase":"bug-reproduction","run":true,"justification":"reproduce the reported defect as a failing test before build hardening","mint":{"prompt":"You write a failing test that reproduces the reported bug.","tier":"balanced","cli":"claude"}}]`

// insertedLeakRelPath is the file cycle-270's inserted phase leaked into the
// main tree (from .evolve/runs/cycle-270/bug-reproduction-report.md).
const insertedLeakRelPath = "go/internal/looppreflight/bug_reproduction_test.go"

// leakMinter is a core.PhaseMinter whose minted runner executes onRun(req) —
// the seam that lets a subtest choose WHERE the minted phase writes (main tree
// vs its worktree). Mirrors fakeMinter's normalization (Optional forced), and
// preserves the parsed spec otherwise so writes_source keeps its mint default.
type leakMinter struct {
	onRun func(req PhaseRequest)
}

func (m leakMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error) {
	spec := cfg.Spec()
	spec.Optional = true
	return spec, &insertedLeakRunner{name: spec.Name, onRun: m.onRun}, nil
}

// insertedLeakRunner (a no-git fake PhaseRunner) lives untagged in
// orchestrator_testfakes_test.go so the fast-tier spinegate test can share it.

// initInsertedLeakRepo creates a real git repo (production gitDirtyPaths runs
// real git against it) with one committed source file, so the leaked test file
// is a NEW untracked path — the cycle-270 shape the tracked-only diff-HEAD
// baseline used to miss.
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

// realLeakWorktree is a WorktreeProvisioner that provisions a REAL `git worktree
// add` off the cycle's projectRoot, OUTSIDE the repo (so worktree writes can
// never appear in the main porcelain — the Test 2 discriminator's invariant).
// It replaces the former fakeWorktree{path: t.TempDir()}: that fake was a plain
// non-git dir, so recoverBuildLeak's `git add -f` of a relocated leak failed
// with "not a git repository" (rc=128), recovery reported failure, and the
// cycle aborted via the "recovery failed" path instead of auto-healing — the
// cycle-535 CI-red root cause. Cleanup is a deliberate no-op so a shipped
// cycle's worktree survives for post-run `git diff HEAD` inspection; t.TempDir
// handles the filesystem teardown. Mirrors buildleak_recover_test.go's
// realWorktree() production topology.
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

func (w *realLeakWorktree) Cleanup(_, _ string) error { return nil }

// insertedLeakOrchestrator wires the full advisory-mint path: routing at
// Advisory+DynamicLLM, a fixedPlanner serving the parsed cycle-270-shaped
// plan, and the leakMinter as registrar. It provisions a REAL git worktree off
// the CycleRequest.ProjectRoot (via realLeakWorktree) so recoverBuildLeak has a
// genuine worktree to relocate + stage a leaked path into. The returned
// provisioner's .path is populated once the cycle provisions the worktree.
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

// TestInsertedPhaseMainTreeLeakRecovers — inbox acceptance #1, on the newer
// leak-recovery contract (cycle-528/529): an advisor-inserted (minted) phase
// that leaks a NEW untracked source file into the MAIN tree is AUTO-RECOVERED
// at the phase boundary — recoverBuildLeak relocates the file into the phase's
// real worktree, stages it, and restores the main tree clean — so the cycle
// completes and ships with the work still visible to the auditor's
// `git diff HEAD`. This preserves cycle-270's guarantee (leaked work must never
// be invisible to audit) via the non-aborting mechanism. Renamed from
// ...Aborts: the guard no longer aborts this shape, it heals it, and a test
// named "Aborts" asserting "ships" would be a maintenance landmine.
func TestInsertedPhaseMainTreeLeakRecovers(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initInsertedLeakRepo(t)
	// mintRan is a plain bool: runner.Run is invoked synchronously on the test
	// goroutine inside RunCycle (no per-phase goroutine), so there is no race.
	// If the dispatch loop ever goes concurrent, -race will surface this.
	mintRan := false
	o, wt := insertedLeakOrchestrator(t, insertedLeakPlanJSON, func(req PhaseRequest) {
		mintRan = true
		// The leak: a new source file in the MAIN tree, not the worktree.
		// Fatalf (not Errorf): a half-executed leak setup must abort the
		// closure immediately, or the test reports a spurious guard RED.
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
	// Auto-heal, not abort: the leaked untracked file is relocated into the
	// worktree and the cycle runs to completion.
	if err != nil {
		t.Fatalf("leak-recovery must auto-heal the main-tree leak and let the cycle complete; got abort: %v", err)
	}
	if !slices.Contains(res.PhasesRun, PhaseShip) {
		t.Errorf("ship never ran — cycle did not complete after recovery (phases=%v)", res.PhasesRun)
	}
	// The leak is no longer in the main tree. Assert the SPECIFIC leaked path is
	// gone (both on disk and to git) — not a blanket "porcelain empty", since a
	// full RunCycle legitimately writes .evolve/runs/ + knowledge-base/cycles/
	// runtime residue into the main tree (isLegitimateMainTreePath state).
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(insertedLeakRelPath))); !os.IsNotExist(statErr) {
		t.Errorf("leaked file must be removed from the main tree; stat err=%v", statErr)
	}
	if st := gitInRepo(t, root, "status", "--porcelain", "-uall"); strings.Contains(st, insertedLeakRelPath) {
		t.Errorf("leaked path %s must not remain in the main tree porcelain; got:\n%s", insertedLeakRelPath, st)
	}
	// Relocated + staged into the worktree, so it is visible to audit's git diff HEAD.
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

// TestInsertedPhaseWorktreeWriteIsClean — inbox acceptance #2 (discriminator):
// the same minted phase writing the same file INSIDE its worktree is isolation
// working as designed — the cycle must continue to ship.
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
