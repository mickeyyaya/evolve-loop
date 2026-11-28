package core_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleidentity"
)

func TestRunCycle_RecordsTheCycleIdentityBeforeAnyPhaseRecord(t *testing.T) {
	t.Parallel()
	id := cycleidentity.Identity{Graph: "g", Contract: "c", Policy: "p"}
	scout := &ctxCaptureRunner{name: string(core.PhaseScout)}
	cfg := core.ResumeIdentityConfig{Stage: config.StageShadow, Compute: func(string) (cycleidentity.Identity, error) { return id, nil }}
	orch, _, ld := newTestOrchestratorOpts(t, newRunners(map[core.Phase]core.PhaseRunner{core.PhaseScout: scout}), core.WithResumeIdentity(cfg))
	res, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "identity-goal", Context: map[string]string{"commit_message": "test commit"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(ld.entries) == 0 || ld.entries[0].Kind != core.LedgerKindCycleIdentity || ld.entries[0].Cycle != res.Cycle {
		t.Fatalf("first ledger record = %+v, want the cycle_identity of cycle %d", ld.entries, res.Cycle)
	}
	if got, derr := cycleidentity.Decode(ld.entries[0].Message); derr != nil || got != id {
		t.Fatalf("recorded identity = %+v, %v", got, derr)
	}
}
