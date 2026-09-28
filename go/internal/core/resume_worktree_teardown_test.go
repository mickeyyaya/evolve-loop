package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// resumeTeardownHarness stages an interrupted cycle the way a real crash
// leaves one: cycle state persisted, ActiveWorktree naming the tree the
// builder was working in, with resume inheriting that path from the
// checkpoint rather than calling worktree.Create.
func resumeTeardownHarness(t *testing.T, verdicts map[Phase]string, opts ...Option) (*fakeStorage, *fakeWorktree, *Orchestrator, string) {
	t.Helper()
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	wt := &fakeWorktree{path: t.TempDir()}
	st := &fakeStorage{
		state: State{LastCycleNumber: 9},
		cycleState: CycleState{
			CycleID:        9,
			WorkspacePath:  ws,
			ActiveWorktree: wt.path,
		},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(verdicts),
		append([]Option{WithWorktreeProvisioner(wt)}, opts...)...)
	return st, wt, o, projectRoot
}

func TestRunCycleFromPhase_PrunesItsWorktreeOnNormalCompletion(t *testing.T) {
	st, wt, o, projectRoot := resumeTeardownHarness(t, nil)

	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseShip), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	if len(wt.cleaned) != 1 || wt.cleaned[0] != wt.path {
		t.Fatalf("a resumed cycle that completed normally never pruned its worktree (cleaned=%v, want exactly [%q]) — "+
			"RunCycleFromPhase registers the lock, run-ID and lease exit actions but not the worktree one, "+
			"so finalizeCycle's preserve decision is computed and then discarded and EVERY resumed cycle leaks its tree",
			wt.cleaned, wt.path)
	}
	if got := st.cycleState.ActiveWorktree; got != "" {
		t.Errorf("persisted cycle state still names the PRUNED worktree %q — the next reader inherits a deleted path (cycle-1278's root cause, on the resume path)", got)
	}
}

func TestRunCycleFromPhase_PreservesAFailedCycleWorktree(t *testing.T) {
	st, wt, o, projectRoot := resumeTeardownHarness(t, map[Phase]string{PhaseAudit: VerdictFAIL})

	res, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 9})
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if res.FinalVerdict != VerdictFAIL {
		t.Fatalf("precondition: the fixture must produce a FAIL verdict (preserveOnVerdict's only true case), got %q", res.FinalVerdict)
	}

	if len(wt.cleaned) != 0 {
		t.Fatalf("a FAILed resumed cycle's worktree was PRUNED (cleaned=%v) — it holds the audited work an operator recovers with `evolve cycle reset`", wt.cleaned)
	}
	if got := st.cycleState.ActiveWorktree; got != wt.path {
		t.Errorf("persisted cycle state lost the PRESERVED worktree (want %q, got %q) — resume/reset reclaim the lane by this path", wt.path, got)
	}
}

func TestRunCycleFromPhase_PreservesAnAbnormallyExitedWorktree(t *testing.T) {
	// Cap the dispatch loop below what the spine needs to reach PhaseEnd, so
	// the run exits via the iteration bound with completed=false.
	st, wt, o, projectRoot := resumeTeardownHarness(t, nil, WithMaxPhaseIterations(2))

	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseScout), CycleID: 9}); err == nil {
		t.Fatal("precondition: the iteration bound must surface an error (the abnormal exit under test)")
	}

	if len(wt.cleaned) != 0 {
		t.Fatalf("an abnormally-exited resumed cycle's worktree was PRUNED (cleaned=%v) — this is the tree `evolve loop --resume` needs to pick up next", wt.cleaned)
	}
	if got := st.cycleState.ActiveWorktree; got != wt.path {
		t.Errorf("persisted cycle state lost the PRESERVED worktree (want %q, got %q)", wt.path, got)
	}
}

func TestGitWorktreeCleanup_RefusesTheProjectRoot(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "the-repository.txt")
	if err := os.WriteFile(sentinel, []byte("live tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlinked alias of the root (macOS t.TempDir already lives under
	// /var → /private/var; this makes the aliasing explicit on every OS).
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, wt := range []string{root, alias, root + string(filepath.Separator)} {
		err := gitWorktree{}.Cleanup(root, wt)
		if err == nil {
			t.Errorf("Cleanup(root, %q) returned nil — the provisioner accepted the project root as a disposable worktree", wt)
		}
		if _, serr := os.Stat(sentinel); serr != nil {
			t.Fatalf("Cleanup(root, %q) DELETED THE REPOSITORY (sentinel gone: %v)", wt, serr)
		}
	}
}

func TestRunCycleFromPhase_ProjectRootCheckpointSurvivesTeardown(t *testing.T) {
	projectRoot := t.TempDir()
	sentinel := filepath.Join(projectRoot, "the-repository.txt")
	if err := os.WriteFile(sentinel, []byte("live tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: projectRoot},
	}
	// No WithWorktreeProvisioner: the default is the real gitWorktree.
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseShip), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("the resumed cycle's exit teardown DELETED THE REPOSITORY it was resumed in (sentinel gone: %v)", err)
	}
	if got := st.cycleState.ActiveWorktree; got != projectRoot {
		t.Errorf("a refused prune must leave the path named for resume/reset (want %q, got %q)", projectRoot, got)
	}
}

func TestSameDirectory(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "not-created-yet")
	for _, tc := range []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", root, root, true},
		{"trailing separator", root, root + string(filepath.Separator), true},
		{"symlink alias resolves by inode", alias, root, true},
		{"nonexistent path is still refused by text", missing, missing, true},
		{"nonexistent vs its parent", missing, root, false},
		{"two different directories", root, t.TempDir(), false},
		{"empty never matches", "", root, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameDirectory(tc.a, tc.b); got != tc.want {
				t.Fatalf("sameDirectory(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
