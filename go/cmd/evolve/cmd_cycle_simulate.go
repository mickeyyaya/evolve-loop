// `evolve cycle run --simulate` is the no-LLM walker. Wires every
// phase with a stub runner that returns PASS without calling out to
// the bridge / Claude / ship.sh. Mirrors the contract of
// scripts/dispatch/cycle-simulator.sh so scripts/parity-audit.sh
// --full can drive both sides through the orchestrator state machine
// and compare phase ordering + artifact shapes without spending money.
//
// What --simulate proves:
//   - The Go orchestrator can sequence all 8 phases without errors
//   - state.json / cycle-state.json / ledger.jsonl transitions are valid
//   - phase-gate hooks (when wired) accept each transition
//
// What it does NOT prove:
//   - LLM output quality (no LLM is invoked)
//   - Real Builder file edits (no source code changes)
//   - Real ship.sh integration (no commit / push)
package main

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolveloop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// simulatePhase satisfies core.PhaseRunner with a deterministic PASS
// response. The Name field carries the phase identity so the
// orchestrator's phase-mapping logic still sees the right name.
type simulatePhase struct {
	name core.Phase
}

func (s *simulatePhase) Name() string { return string(s.name) }

func (s *simulatePhase) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{
		Phase:        string(s.name),
		Verdict:      core.VerdictPASS,
		ArtifactsDir: fmt.Sprintf("%s/runs/cycle-%d", req.Workspace, req.Cycle),
		CostUSD:      0.0,
		DurationMS:   0,
	}, nil
}

// wireSimulateOrchestrator returns an orchestrator with every phase
// replaced by a simulatePhase. Storage + ledger remain real so the
// state machine state mutates correctly — that's the whole point of
// the simulate path (drive transitions without spending money).
// simulatePhases is the set of phases the no-LLM `-simulate` harness registers a
// PASS runner for. It must cover every phase the canonical order
// (phaseorder.HardcodedOrder) or the core phase set can route to — the list was
// stale (missing build-planner/swarm-plan/plan-review/tester/retrospective/memo),
// so a full-cycle walk hit "no runner registered for phase build-planner". Extra
// runners are harmless. TestSimulatePhases_CoversCanonicalOrder guards this.
func simulatePhases() []core.Phase {
	return []core.Phase{
		core.PhaseIntent,
		core.PhaseScout,
		core.PhaseTriage,
		core.Phase("plan-review"),
		core.PhaseTDD,
		core.PhaseBuildPlanner,
		core.PhaseSwarmPlan,
		core.PhaseBuild,
		core.Phase("tester"),
		core.PhaseAudit,
		core.PhaseShip,
		core.PhaseRetro,
		core.Phase("retrospective"),
		core.Phase("memo"),
	}
}

func wireSimulateOrchestrator(_, evolveDir string) *core.Orchestrator {
	phases := simulatePhases()
	runners := make(map[core.Phase]core.PhaseRunner, len(phases))
	for _, p := range phases {
		runners[p] = &simulatePhase{name: p}
	}

	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir)
	return core.NewOrchestrator(st, ld, runners)
}
