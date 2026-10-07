//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestIsLegitimateMainTreePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
		note string
	}{
		{".evolve/runs/cycle-274/triage-report.md", true, "workspace run artifact"},
		{".evolve/state.json", true, "cycle state"},
		{".evolve/ledger.jsonl", true, "ledger"},
		{".evolve", true, "top-level .evolve dir"},
		{"go/subdir/.evolve/guards.log", true, "nested .evolve guard log (cycle-176 precedent)"},
		{"go/evolve", true, "tracked release binary"},
		{"go/bin/evolve", true, "gitignored build binary"},
		{"go/acs/cycle274/", true, "bare worktree dir entry (trailing slash)"},
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

// fakeGitDirty fakes the guard's git query without a real repo: baseline on
// the first call (snapshot), afterLeak on every call after (check).
type fakeGitDirty struct {
	callCount int
	baseline  []string
	afterLeak []string
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

func TestGuardCatchesInsertedPhaseLeak(t *testing.T) {
	cases := []struct {
		name      string
		leakPhase Phase
		leakPath  string // path that appears as untracked after the phase
	}{
		{
			name:      "untracked_source_file_after_scout",
			leakPhase: PhaseScout,
			leakPath:  "go/internal/looppreflight/bug_reproduction_test.go",
		},
		{
			name:      "untracked_source_file_after_triage",
			leakPhase: PhaseTriage,
			leakPath:  "go/internal/core/advisor_injected_feature.go",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dirty := &fakeGitDirty{
				baseline:  []string{},
				afterLeak: []string{tc.leakPath},
			}
			runners := minimalRunners(tc.leakPhase, &leakInjector{name: tc.leakPhase})
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
				WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
				WithGitDirtyPaths(dirty.Fn()),
			)
			_, err := o.RunCycle(context.Background(), CycleRequest{
				ProjectRoot: t.TempDir(),
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

func TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase(t *testing.T) {
	breakerPath := ".evolve/contract-gate-breaker.json"
	// dirtyFn flips only after the phase runs, so the leak attributes to the
	// worktree phase rather than fakeGitDirty's fixed first-call flip.
	leakPhaseRan := false
	dirtyFn := func(_ context.Context, _ string) ([]string, error) {
		if leakPhaseRan {
			return []string{breakerPath}, nil
		}
		return nil, nil
	}
	// tdd stands in for a worktree phase; build sits behind build-planner,
	// which this harness doesn't register.
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
	if err != nil && strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("guard must not fire on orchestrator self-write during a worktree phase; got: %v", err)
	}
}

func TestGuardCatchesDeliverableRenameSmuggle_WorktreePhase(t *testing.T) {
	leakPhaseRan := false
	dirtyFn := func(_ context.Context, _ string) ([]string, error) {
		if leakPhaseRan {
			return []string{
				".evolve/commit-prefix-scope.json",
				".evolve/commit-prefix-scope.renamed.json",
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

func TestGuardIgnoresLegitimateWorkspaceWrite(t *testing.T) {
	workspacePath := ".evolve/runs/cycle-1/triage-report.md"
	dirty := &fakeGitDirty{
		baseline:  []string{},
		afterLeak: []string{workspacePath},
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
	if err != nil && strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("guard must not fire on legitimate .evolve/ workspace write; got: %v", err)
	}
}

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

func TestGuardIgnoresScoutEvalMaterialization(t *testing.T) {
	root := initAuditLeakRepo(t)
	const slug = "ledger-seal-io-coverage"
	writeLanePin(t, RunWorkspacePath(root, 1), slug)
	evalPath := ".evolve/evals/" + slug + ".md"
	var copied atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseScout] = &tddLeakRunner{name: string(PhaseScout), onRun: func() {
		writeTreeFile(t, root, evalPath, "scout copy in the main tree\n")
		copied.Store(true)
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err != nil {
		t.Fatalf("a scout's copy of its own eval in the main tree must not abort the cycle: %v", err)
	}
	if !slices.Contains(res.PhasesRun, PhaseShip) {
		t.Fatalf("the cycle must run to ship; phases=%v", res.PhasesRun)
	}
	if !copied.Load() {
		t.Fatal("the scout never wrote its main-tree copy")
	}
	requireAbsent(t, root, evalPath)
}

func TestGuardStillChargesAnEvalNoLiveSiblingHolds(t *testing.T) {
	root := initAuditLeakRepo(t)
	const strayEval = ".evolve/evals/another-item.md"
	writeLanePin(t, RunWorkspacePath(root, 1), "router-silent-errors")
	runners := buildRunners(nil)
	runners[PhaseShip] = &tddLeakRunner{name: string(PhaseShip), onRun: func() {
		writeTreeFile(t, root, strayEval, "written during a phase leak recovery never runs for\n")
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err == nil || !strings.Contains(err.Error(), "tree-diff") || !strings.Contains(err.Error(), strayEval) {
		t.Fatalf("an eval no live sibling holds must still be charged to the phase that wrote it; got: %v", err)
	}
}

func TestRecoveryFailsAPhaseThatLeftAnEvalNoLiveLaneHolds(t *testing.T) {
	root := initAuditLeakRepo(t)
	const strayEval = ".evolve/evals/another-item.md"
	writeLanePin(t, RunWorkspacePath(root, 1), "router-silent-errors")
	runners := buildRunners(nil)
	runners[PhaseTriage] = &tddLeakRunner{name: string(PhaseTriage), onRun: func() {
		writeTreeFile(t, root, strayEval, "written by this lane's triage\n")
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("recovery must fail the phase that left a keyed path no live lane holds; got: %v", err)
	}
	requireTreeFile(t, root, strayEval, "written by this lane's triage\n")
}

func TestGuardRecoversCatalogWritesSourcePhaseLeak(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initAuditLeakRepo(t)
	runners := buildRunners(nil)
	runners[PhaseScout] = &auditLeakRunner{name: string(PhaseScout), onRun: func() {
		if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte(auditLeakChurn), 0o644); err != nil {
			t.Errorf("scout leak write: %v", err)
		}
	}}
	// A catalog-declared source-writer (not the hardcoded WorktreePhase literal
	// set) so o.worktreePhase(scout) is true only via catalog consultation.
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "scout", WritesSource: true}})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(gitWorktree{}),
		WithCatalog(cat),
	)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("catalog source-writer (scout, writes_source:true) leak must be recovered, not aborted; got: %v", err)
	}
	if got := auditLeakReadFile(t, filepath.Join(root, "docs", "note.md")); got != auditLeakNoteV1 {
		t.Errorf("docs/note.md in main = %q, want committed %q (leak must be relocated, main restored)", got, auditLeakNoteV1)
	}
	shipRan := false
	for _, p := range res.PhasesRun {
		if p == PhaseShip {
			shipRan = true
		}
	}
	if !shipRan {
		t.Errorf("ship never ran — cycle aborted at the scout leak instead of recovering (phases=%v)", res.PhasesRun)
	}
}

func TestGuardStillAbortsNonSourcePhaseLeak(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initAuditLeakRepo(t)
	runners := buildRunners(nil)
	runners[PhaseAudit] = &auditLeakRunner{name: string(PhaseAudit), onRun: func() {
		if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte(auditLeakChurn), 0o644); err != nil {
			t.Errorf("audit leak write: %v", err)
		}
	}}
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "scout", WritesSource: true}})
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(gitWorktree{}),
		WithCatalog(cat),
	)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err == nil {
		t.Fatalf("non-source phase (audit, writes_source:false) source leak must abort; got nil (phases=%v)", res.PhasesRun)
	}
	if !strings.Contains(err.Error(), "tree-diff") {
		t.Errorf("abort must come from the tree-diff guard; got: %v", err)
	}
}
