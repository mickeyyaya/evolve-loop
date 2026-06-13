package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// orchestrator_guard_test.go — cycle-274 G task: inserted-phase tree-diff guard gap.
//
// Background (cycle-270 defect): the tree-diff guard was gated on
// `phaseWorktree != ""`, so non-worktree phases (scout, triage, retro, and
// advisor-inserted minted phases) ran with no snapshot — any untracked source
// file they wrote to the main tree slipped through to audit and potentially to
// ship. The fix (G-B): drop the worktree gate, snapshot every phase, filter
// legitimate `.evolve/` workspace writes via isLegitimateMainTreePath.
//
// Test map:
//   TestIsLegitimateMainTreePath     — R9: pure classifier unit test (no git, ≥2 sub-cases)
//   TestGuardCatchesInsertedPhaseLeak — R5+R6: cycle aborts when untracked source leak detected (≥2 sub-cases)
//   TestGuardIgnoresLegitimateWorkspaceWrite — R7: legitimate .evolve/ workspace write does NOT trip the guard

// --- TestIsLegitimateMainTreePath ---

func TestIsLegitimateMainTreePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
		note string
	}{
		// Legitimate: .evolve/ workspace paths (runs, state, runtime logs)
		{".evolve/runs/cycle-274/triage-report.md", true, "workspace run artifact"},
		{".evolve/state.json", true, "cycle state"},
		{".evolve/ledger.jsonl", true, "ledger"},
		{".evolve", true, "top-level .evolve dir"},
		{"go/subdir/.evolve/guards.log", true, "nested .evolve guard log (cycle-176 precedent)"},
		// Legitimate: build artifacts
		{"go/evolve", true, "tracked release binary"},
		{"go/bin/evolve", true, "gitignored build binary"},
		// Legitimate: bare directory entry from -uall
		{"go/acs/cycle274/", true, "bare worktree dir entry (trailing slash)"},
		// NOT legitimate: real source files
		{"go/internal/looppreflight/bug_reproduction_test.go", false, "cycle-270 leak path"},
		{"go/internal/core/new_feature.go", false, "source file leak"},
		{"docs/architecture/new-adr.md", false, "doc leak"},
		{"go/acs/cycle274/predicates_test.go", false, "ACS predicate source"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			if got := isLegitimateMainTreePath(tc.path); got != tc.want {
				t.Errorf("isLegitimateMainTreePath(%q)=%v, want %v (%s)", tc.path, got, tc.want, tc.note)
			}
		})
	}
}

// --- TestGuardCatchesInsertedPhaseLeak ---

// leakInjector is a fake phase runner that reports PASS but also calls an
// optional side-effect (e.g. writing a source file leak to the main tree).
type leakInjector struct {
	name  Phase
	onRun func(req PhaseRequest)
}

func (r *leakInjector) Name() string { return string(r.name) }
func (r *leakInjector) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.onRun != nil {
		r.onRun(req)
	}
	return PhaseResponse{Phase: string(r.name), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// fakeGitDirty is a test seam for the tree-diff guard: before the first call
// it returns the clean baseline; after the first call it returns the dirty set
// (simulating a phase that wrote a new untracked file). This avoids the need
// for a real git repo while exercising the full guard + isLegitimateMainTreePath logic.
type fakeGitDirty struct {
	callCount int
	baseline  []string // returned on first call (snapshot)
	afterLeak []string // returned on subsequent calls (check)
}

func (f *fakeGitDirty) Fn() func(ctx context.Context, repoRoot string) ([]string, error) {
	return func(_ context.Context, _ string) ([]string, error) {
		f.callCount++
		if f.callCount == 1 {
			return f.baseline, nil
		}
		return f.afterLeak, nil
	}
}

// minimalRunners builds a runners map where all spine phases pass trivially.
// The provided phase runner overrides the default for that phase.
func minimalRunners(override Phase, r PhaseRunner) map[Phase]PhaseRunner {
	pass := func(ph Phase) PhaseRunner { return &leakInjector{name: ph} }
	m := map[Phase]PhaseRunner{
		PhaseScout:  pass(PhaseScout),
		PhaseTriage: pass(PhaseTriage),
		PhaseTDD:    pass(PhaseTDD),
		PhaseBuild:  pass(PhaseBuild),
		PhaseAudit:  pass(PhaseAudit),
		PhaseShip:   pass(PhaseShip),
		PhaseRetro:  pass(PhaseRetro),
	}
	if r != nil {
		m[override] = r
	}
	return m
}

// TestGuardCatchesInsertedPhaseLeak tests that the guard fires for any
// non-worktree phase that writes a new untracked source file into the main
// tree. Two sub-cases exercise distinct aspects of R5 (any-phase) and R6
// (untracked granularity).
func TestGuardCatchesInsertedPhaseLeak(t *testing.T) {
	cases := []struct {
		name      string
		leakPhase Phase
		leakPath  string // path that appears as untracked after the phase
	}{
		{
			// R6: an untracked source file (never in git) shows up after scout
			// (a spine, non-worktree phase) — the original cycle-270 escape route.
			name:      "untracked_source_file_after_scout",
			leakPhase: PhaseScout,
			leakPath:  "go/internal/looppreflight/bug_reproduction_test.go",
		},
		{
			// R5: a different spine non-worktree phase (triage) also triggers —
			// the guard fires regardless of phase identity.
			name:      "untracked_source_file_after_triage",
			leakPhase: PhaseTriage,
			leakPath:  "go/internal/core/advisor_injected_feature.go",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// fakeGitDirty: before the leaking phase → clean; after → leakPath
			dirty := &fakeGitDirty{
				baseline:  []string{},            // clean before
				afterLeak: []string{tc.leakPath}, // new untracked file after
			}
			runners := minimalRunners(tc.leakPhase, &leakInjector{name: tc.leakPhase})
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
				WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
				WithGitDirtyPaths(dirty.Fn()),
			)
			_, err := o.RunCycle(context.Background(), CycleRequest{
				ProjectRoot: t.TempDir(), // fake root; gitDirtyPaths is injected
				GoalHash:    "g",
			})
			if err == nil {
				t.Fatal("expected cycle abort for main-tree source leak; got nil error")
			}
			if !strings.Contains(err.Error(), "tree-diff") {
				t.Errorf("abort must come from the tree-diff guard; got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.leakPath) {
				t.Errorf("abort error must name the leaked path %q; got: %v", tc.leakPath, err)
			}
		})
	}
}

// TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase pins the CI regression
// from the cycle-274 salvage: during a WORKTREE phase (tdd/build), the
// orchestrator-side contract gate writes its own untracked runtime state
// (.evolve/contract-gate-breaker.json) into the main tree. That is not a
// phase escape — recoverBuildLeak already classifies it legitimate and skips
// it — so the tree-diff guard must apply the same classification instead of
// aborting the cycle (guard and recovery must agree on one vocabulary).
func TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase(t *testing.T) {
	breakerPath := ".evolve/contract-gate-breaker.json"
	// Phase-aware seam: the dirty set appears only once BUILD has run, so the
	// leak is attributed to the worktree phase (fakeGitDirty's first-call flip
	// would surface it at scout, a non-worktree phase, and miss the branch
	// under test).
	leakPhaseRan := false
	dirtyFn := func(_ context.Context, _ string) ([]string, error) {
		if leakPhaseRan {
			return []string{breakerPath}, nil // gate wrote its breaker during the phase
		}
		return nil, nil // clean before the worktree phase
	}
	// tdd: a worktree phase (runsInWorktree) reachable with minimalRunners —
	// build sits behind build-planner, which this harness does not register.
	runners := minimalRunners(PhaseTDD, &leakInjector{
		name:  PhaseTDD,
		onRun: func(PhaseRequest) { leakPhaseRan = true },
	})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithGitDirtyPaths(dirtyFn),
	)
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "g",
	})
	// The guard must NOT have fired. Any other error (e.g. ship gate) is acceptable.
	if err != nil && strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("guard must not fire on orchestrator self-write during a worktree phase; got: %v", err)
	}
}

// TestGuardCatchesDeliverableRenameSmuggle_WorktreePhase pins the companion
// strict case: when both rename sides reach the guard (the porcelain
// emission contract is pinned separately by
// TestDefaultGitDirtyPaths_RenameEmitsBothSides), the deliverable old path
// keeps the guard armed even though the look-alike new path classifies as
// legitimate runtime state — so a worktree phase cannot smuggle a
// deliverable out via rename.
func TestGuardCatchesDeliverableRenameSmuggle_WorktreePhase(t *testing.T) {
	leakPhaseRan := false
	dirtyFn := func(_ context.Context, _ string) ([]string, error) {
		if leakPhaseRan {
			return []string{
				".evolve/commit-prefix-scope.json",         // rename source (deliverable)
				".evolve/commit-prefix-scope.renamed.json", // rename target (look-alike)
			}, nil
		}
		return nil, nil
	}
	runners := minimalRunners(PhaseTDD, &leakInjector{
		name:  PhaseTDD,
		onRun: func(PhaseRequest) { leakPhaseRan = true },
	})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithGitDirtyPaths(dirtyFn),
	)
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "g",
	})
	if err == nil {
		t.Fatal("expected cycle abort for deliverable rename leak; got nil error")
	}
	if !strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("abort must come from the tree-diff guard; got: %v", err)
	}
}

// TestGuardIgnoresLegitimateWorkspaceWrite tests R7: a non-worktree phase that
// writes only its .evolve/runs/... workspace artifact must NOT trip the guard.
// The cycle must NOT fail due to "tree-diff" after the workspace write.
func TestGuardIgnoresLegitimateWorkspaceWrite(t *testing.T) {
	// Simulate triage writing its workspace report (.evolve/runs/cycle-1/triage-report.md).
	// The guard sees this as a new path but isLegitimateMainTreePath returns true,
	// so the cycle continues without a tree-diff abort.
	workspacePath := ".evolve/runs/cycle-1/triage-report.md"
	dirty := &fakeGitDirty{
		baseline:  []string{},              // clean before triage
		afterLeak: []string{workspacePath}, // triage wrote its report
	}
	runners := minimalRunners(PhaseTriage, &leakInjector{name: PhaseTriage})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithGitDirtyPaths(dirty.Fn()),
	)
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "g",
	})
	// The guard must NOT have fired. Any other error (e.g. ship gate) is acceptable.
	if err != nil && strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("guard must not fire on legitimate .evolve/ workspace write; got: %v", err)
	}
}

// TestDefaultGitDirtyPaths_RenameEmitsBothSides pins the porcelain parsing
// contract the rename-smuggle guard case depends on: a staged rename must
// surface BOTH the old and the new path, so a deliverable renamed to a
// look-alike name cannot vanish from the guard's view.
func TestDefaultGitDirtyPaths_RenameEmitsBothSides(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Parallel()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "tracked.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.json")
	run("commit", "-q", "-m", "seed")
	run("mv", "tracked.json", "renamed.json")

	paths, err := defaultGitDirtyPaths(context.Background(), root)
	if err != nil {
		t.Fatalf("defaultGitDirtyPaths: %v", err)
	}
	got := map[string]bool{}
	for _, p := range paths {
		got[p] = true
	}
	if !got["tracked.json"] || !got["renamed.json"] {
		t.Errorf("rename must emit both sides; got %v", paths)
	}
}

// TestGuardIgnoresScoutEvalMaterialization pins the soak-#6 cycle-319 defect:
// scout's contract (internal/evalgate/materialization.go) is to write the
// SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN
// tree, where Gate A reads them. A later cycle iterating the same coverage
// target re-materializes the same slug, MODIFYING the prior cycle's committed
// eval (318→319 ledger-seal-io-coverage) — a main-tree write the tree-diff
// guard wrongly flagged as a deliverable leak and aborted the cycle. Scout's
// eval materialization is its JOB, not a deliverable escape.
func TestGuardIgnoresScoutEvalMaterialization(t *testing.T) {
	evalPath := ".evolve/evals/ledger-seal-io-coverage.md"
	dirty := &fakeGitDirty{baseline: []string{}, afterLeak: []string{evalPath}}
	runners := minimalRunners(PhaseScout, &leakInjector{name: PhaseScout})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithGitDirtyPaths(dirty.Fn()),
	)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	// Non-vacuous: the cycle must ADVANCE PAST scout's guard. minimalRunners
	// registers only scout, so the next spine phase has no runner — the cycle
	// errors with "no runner", proving the scout guard passed (not a tree-diff
	// abort, and the eval path is never named as a leak).
	if err == nil || !strings.Contains(err.Error(), "no runner") {
		t.Fatalf("expected the cycle to advance past scout's guard to a no-runner phase; got: %v", err)
	}
	if strings.Contains(err.Error(), "tree-diff") || strings.Contains(err.Error(), evalPath) {
		t.Errorf("guard must NOT fire on scout eval materialization; got: %v", err)
	}
}

// TestIsScoutEvalMaterialization pins the exemption's scope so it cannot widen
// into a deliverable-leak loophole: ONLY scout + .evolve/evals/ is exempt. A
// non-scout phase writing an eval, or scout writing anything outside evals/,
// is NOT exempt and still classifies as a leak.
func TestIsScoutEvalMaterialization(t *testing.T) {
	cases := []struct {
		phase Phase
		path  string
		want  bool
	}{
		{PhaseScout, ".evolve/evals/ledger-seal-io-coverage.md", true},
		{PhaseScout, ".evolve/evals/x.md", true},
		{PhaseScout, ".evolve/evals/notamarkdown.sh", false}, // only <slug>.md is the contract
		{PhaseTriage, ".evolve/evals/smuggled.md", false},    // only scout materializes evals
		{PhaseBuild, ".evolve/evals/smuggled.md", false},
		{PhaseScout, ".evolve/phases/x/phase.json", false}, // other deliverables not exempt
		{PhaseScout, "go/internal/core/leak.go", false},    // source leak from scout still fires
	}
	for _, tc := range cases {
		if got := isScoutEvalMaterialization(tc.phase, tc.path); got != tc.want {
			t.Errorf("isScoutEvalMaterialization(%q,%q)=%v, want %v", tc.phase, tc.path, got, tc.want)
		}
	}
}
