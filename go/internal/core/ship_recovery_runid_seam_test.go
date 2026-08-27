package core_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// ship_recovery_runid_seam_test.go — cycle-1571 H3, the WIRING PROOF (§3.3).
//
// PR #503 widened WithCompositionSnapshot from (ctx, worktree) to
// (ctx, worktree, runID) so the composition reader could refuse a foreign run's
// audit. Nothing asserted the value ARRIVES: every existing fixture either
// calls latestAuditEntry directly or uses a stub that ignores its arguments.
// Replace cs.RunID with "" at either production call site and the entire suite
// stays green while the hardening is permanently dark — the cycle-1064 trap the
// ship package documents in its own comments ("Options.ManifestGate was
// silently never assigned here, leaving the manifest gate permanently shadow").
//
// The proof: capture what the closure actually receives on the live path
// (recoverFromShipError -> compositionCarryForward) and compare it to the
// RunID the orchestrator persisted for that cycle.

func TestCompositionSnapshot_ReceivesThisRunsRunID(t *testing.T) {
	dir, preDiff := initCleanRebaseRepoT(t)
	patchID, err := ledger.PatchID(preDiff)
	if err != nil {
		t.Fatalf("compute fixture patch-id: %v", err)
	}

	var gotRunIDs []string
	st := &recStorage{}
	ship := &shipErrorStub{
		name:      "ship",
		failFirst: 1,
		errOnFail: core.NewShipError(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip, "peer landed during audit->ship gap"),
	}
	o := core.NewOrchestrator(st, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:  ship,
		core.PhaseAudit: &countingRunner{name: "audit"},
	}),
		core.WithWorktreeProvisioner(fixedWorktree{dir: dir}),
		core.WithCompositionSnapshot(func(_ context.Context, _ string, runID string) (core.CompositionAuditSnapshot, error) {
			gotRunIDs = append(gotRunIDs, runID)
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         preDiff,
				PatchID:      patchID,
			}, nil
		}),
		core.WithCompositionGateRunner(func(context.Context, string) map[string]string { return greenComposedGateResults() }),
		core.WithCompositionVerdictWriter(func(string, core.CompositionVerdictInput) error { return nil }),
	)

	if _, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "merge-rung0-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	// "Never called" is a DIFFERENT failure from "called with the wrong value":
	// if the seam is not reached the assertion below would vacuously pass.
	if len(gotRunIDs) == 0 {
		t.Fatal("the composition snapshot seam was never invoked — this test proves nothing until it is")
	}

	wantRunID := st.cycleStateRunID()
	if wantRunID == "" {
		t.Fatal("precondition: the orchestrator persisted no RunID for this cycle (CA.5 regression)")
	}
	for i, got := range gotRunIDs {
		if got != wantRunID {
			t.Errorf("composition snapshot call[%d] received runID %q, want this run's %q — "+
				"run identity must reach the seam or #503's foreign-run refusal silently degrades to latest-any",
				i, got, wantRunID)
		}
	}
}

// cycleStateRunID reads the RunID the orchestrator persisted, under the same
// lock WriteCycleState takes, so the assertion stays clean under -race.
func (s *recStorage) cycleStateRunID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cs.RunID
}
