package core

// resume_worktree_teardown_test.go — the resume mirror of
// cyclerun_worktree_teardown_test.go.
//
// newCycleRun builds a four-entry LIFO cleanup stack (lock release, run-ID
// clear, worktree prune/preserve, lease stop) and hands it back as ONE closure;
// RunCycle defers it. RunCycleFromPhase registers three of those four exit
// actions by hand — release, currentRunID.Store(""), stopLease — and no
// worktree teardown at all.
//
// The consequence is not that the flag is wrong: completeCycle IS shared, so
// finalizeCycle computes preserveOnVerdict on the resume path too and stores it
// on the cycleRun. Nothing on the resume path ever READS it. Every resumed
// cycle therefore leaks its worktree, whatever its verdict — 26 stale
// cycle-* trees were sitting in the live runtime when this was written.
//
// These tests drive the REAL entrypoint (RunCycleFromPhase), not the teardown
// helper, because a helper that exists and is never deferred is exactly the
// defect. The assertion is on fakeWorktree.cleaned — what the provisioner was
// actually asked to do.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// resumeTeardownHarness stages an interrupted cycle the way a real crash leaves
// one: cycle state persisted, ActiveWorktree naming the tree the builder was
// working in. Resume inherits that path from the checkpoint — it never calls
// worktree.Create — so the tree under test is the one the crashed cycle made.
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

// TestRunCycleFromPhase_PrunesItsWorktreeOnNormalCompletion is the crux. A
// resumed cycle that completes normally with a non-FAIL verdict has had its
// work merged by ship, exactly as on the fresh path — so the worktree is spent
// and must be pruned. Leaking it is not cosmetic: every leaked tree is a full
// checkout, and the next reader of the persisted state still sees a path this
// cycle is done with.
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

// TestRunCycleFromPhase_PreservesAFailedCycleWorktree is the narrowness guard,
// and it is the one that matters most: this worktree holds audited, possibly
// uncommitted work, and `evolve loop --resume` / `evolve cycle reset` reclaim
// the lane BY that path. A teardown that pruned unconditionally would trade a
// leak for the cycle-7 lost-work incident.
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

// TestRunCycleFromPhase_PreservesAnAbnormallyExitedWorktree is the second
// narrowness axis: !completedNormally preserves regardless of verdict, because
// a cycle that died mid-phase has work nobody has adjudicated yet.
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

// TestGitWorktreeCleanup_RefusesTheProjectRoot pins the invariant at the
// component that performs the destructive operation. It was found by an
// existing test failing, not by design: the fresh path's wtPath always comes
// from worktree.Create, but resume reads ActiveWorktree from a persisted
// checkpoint, and that field CAN name the project root — the ship package
// already branches on exactly that shape ("a cycle whose worktree resolves to
// the project root" routes to shipDirect). gitWorktree.Cleanup runs
// `git worktree remove --force` (a harmless failure on the main tree) and
// then os.RemoveAll UNCONDITIONALLY: without this refusal, disposing of such
// a checkpoint deletes the repository. The refusal lives in the provisioner,
// beside deleteCycleBranch's "cycle-" gate, so every caller of Cleanup is
// covered — not only the orchestrator's exit path.
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

// TestRunCycleFromPhase_ProjectRootCheckpointSurvivesTeardown drives the whole
// path — a resumed cycle whose checkpoint names the project root, completing
// normally, through the REAL provisioner — and asserts the repository is still
// there afterwards. The orchestrator-level tests above use a fake that only
// records requests; this one pins the syscall.
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

// TestSameDirectory pins the refusal predicate's two halves and its
// fail-safe direction: identity by inode when both paths exist, and by text
// when one does not, so a path that cannot be stat'ed is still refused when
// it is textually the root.
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
