package core_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Fresh explanation contracts deliberately supersede the legacy composition
// carry-forward shortcut: a new base invalidates the Build-authored rationale,
// so recovery must rebuild and re-audit even when the lane patch-id is stable.
// The direct run-ID wiring proof lives in composition_carryforward_wired_test.

func TestFreshExplanationRebase_BypassesLegacyCompositionCarryForward(t *testing.T) {
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
	builder := &explanationWritingRunner{}
	o := core.NewOrchestrator(st, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:  ship,
		core.PhaseAudit: &countingRunner{name: "audit"},
		core.PhaseBuild: builder,
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
	if len(gotRunIDs) != 0 {
		t.Fatalf("fresh explanation rebase used legacy composition carry-forward with run IDs %v", gotRunIDs)
	}
	if builder.calls < 2 {
		t.Fatalf("fresh explanation Build ran %d time(s), want initial Build plus post-rebase rebuild", builder.calls)
	}
}
