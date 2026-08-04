package core

// cyclerun_worktree_teardown_test.go — cycle-1278
// `retro-fleet-stale-worktree-fallback`, AC2 (the root-cause companion).
//
// cs.ActiveWorktree = wtPath (cyclerun.go:456) is the SOLE assignment; nothing
// clears it. When the lane teardown callback prunes the worktree
// (o.worktree.Cleanup, cyclerun.go:471) the persisted cycle state keeps pointing
// at the now-deleted directory, and the next dispatch to read that file hands the
// stale path to the bridge — where isDir() refuses the launch. Widening
// retroWorktree's fallback (AC1) contains the symptom; clearing the field at
// teardown removes the source.
//
// These drive the REAL production seam: newCycleRun is what RunCycle calls, and
// the closure it returns is the one RunCycle defers. The assertion is on the
// PERSISTED cycle state (fakeStorage.cycleState — the last WriteCycleState), not
// on an in-memory local, because the persisted file is what a later dispatch
// actually reads.

import (
	"context"
	"testing"
)

// teardownHarness builds an orchestrator over the standard fakes and returns the
// storage, the worktree provisioner, and the cleanup closure newCycleRun handed
// back — i.e. exactly what RunCycle defers.
func teardownHarness(t *testing.T) (*fakeStorage, *fakeWorktree, func(preserve, completedNormally bool)) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 1277}} // cycle 1278
	wt := &fakeWorktree{path: t.TempDir()}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(wt))

	_, cleanup, err := o.newCycleRun(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("newCycleRun: %v", err)
	}
	if cleanup == nil {
		t.Fatal("newCycleRun returned a nil cleanup closure — the teardown path under test does not exist")
	}
	if st.cycleState.ActiveWorktree != wt.path {
		t.Fatalf("precondition: cycle state should carry the provisioned worktree %q, got %q", wt.path, st.cycleState.ActiveWorktree)
	}
	return st, wt, cleanup
}

// TestCycleRunTeardown_ClearsActiveWorktreeAfterPrune is the crux (AC2). Once the
// worktree is actually pruned, the persisted cycle state must no longer name it —
// otherwise the very next reader (retro's dispatch, resume, checkpoint) inherits a
// path that no longer exists.
func TestCycleRunTeardown_ClearsActiveWorktreeAfterPrune(t *testing.T) {
	st, wt, cleanup := teardownHarness(t)

	cleanup(false /*preserve*/, true /*completedNormally*/)

	if len(wt.cleaned) != 1 || wt.cleaned[0] != wt.path {
		t.Fatalf("precondition: teardown should have pruned %q exactly once, cleaned=%v", wt.path, wt.cleaned)
	}
	if got := st.cycleState.ActiveWorktree; got != "" {
		t.Fatalf("persisted cycle state still names the PRUNED worktree %q after teardown — the next dispatch hands this deleted path to the bridge, whose isDir() guard refuses the launch (the cycle-1255 CRITICAL's root cause)", got)
	}
}

// TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree is the negative axis,
// and it is load-bearing: `evolve loop --resume` and `evolve cycle reset` reclaim
// a preserved lane BY that path. Clearing it unconditionally would trade a stale
// path for permanently orphaned audited work — the cycle-7 lost-work incident.
func TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		preserve, completedNormally bool
	}{
		{"ship-stage failure preserves", true, true},
		{"abnormal exit preserves", false, false},
		{"both", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, wt, cleanup := teardownHarness(t)

			cleanup(tc.preserve, tc.completedNormally)

			if len(wt.cleaned) != 0 {
				t.Fatalf("precondition: a preserved worktree must not be pruned, cleaned=%v", wt.cleaned)
			}
			if got := st.cycleState.ActiveWorktree; got != wt.path {
				t.Fatalf("persisted cycle state lost the PRESERVED worktree (want %q, got %q) — resume/reset reclaim the lane by this path; clearing it orphans audited work", wt.path, got)
			}
		})
	}
}
