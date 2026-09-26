package core

import (
	"context"
	"testing"
)

func teardownHarness(t *testing.T) (*fakeStorage, *fakeWorktree, func(preserve, completedNormally bool)) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 1277}}
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
