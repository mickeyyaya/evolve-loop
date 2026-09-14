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
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// simulatePhase satisfies core.PhaseRunner with a deterministic PASS
// response. The Name field carries the phase identity so the
// orchestrator's phase-mapping logic still sees the right name.
type simulatePhase struct {
	name core.Phase
}

func (s *simulatePhase) Name() string { return string(s.name) }

func (s *simulatePhase) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	// The contracted report stub: a no-LLM walk still hands the floors a
	// deliverable to grade (docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).
	if req.Workspace != "" {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, fmt.Errorf("simulate %s: workspace: %w", s.name, err)
		}
		// The sentinel and the declaration are rendered by their owners
		// (phasecontract, explanationdocs) so the walk keeps proving the
		// contract the real producers meet when either grammar moves.
		stub := "# " + string(s.name) + " (simulate)\n\n" + phasecontract.RenderVerdictSentinel(string(s.name), core.VerdictPASS) + "\n"
		if s.name == core.PhaseBuild {
			// The build handoff floor grades the explanation-documentation
			// declaration; a walk produces no Build diff, so it declares that.
			stub += "\n" + explanationdocs.RenderNotApplicableDeclaration("simulate walk — the no-LLM plumbing check produces no Build diff")
		}
		if err := os.WriteFile(filepath.Join(req.Workspace, string(s.name)+"-report.md"), []byte(stub), 0o644); err != nil {
			return core.PhaseResponse{}, fmt.Errorf("simulate %s: report stub: %w", s.name, err)
		}
	}
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

// wireSimulateOrchestrator builds the --simulate root: the simulate runners,
// the plane's storage and ledger, and the SAME signal topology as the
// production root (newRootSignalCenter, the observed ledger, the orchestrator
// registered as listener) — so the recorder's and the seal's warnings render
// on the console here exactly as in a real cycle. Unit 01's architecture
// review (HIGH-1) found a Center-less simulate root had silenced the six
// warnings the deleted stderr lines used to print. No bridge: the parity walk
// launches no sessions.
func wireSimulateOrchestrator(projectRoot, evolveDir string, console io.Writer) orchDeps {
	phases := simulatePhases()
	runners := make(map[core.Phase]core.PhaseRunner, len(phases))
	for _, p := range phases {
		runners[p] = &simulatePhase{name: p}
	}

	signals := newRootSignalCenter(projectRoot, evolveDir, console)
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir, ledger.WithSignals(signals))
	// A --simulate walk must never mutate the operator's repository: no cycle
	// worktree or branch (the phases never write, so the root is read in place)
	// and no `dossier: cycle-N closeout` commit (the record is still written).
	// docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md.
	return orchDeps{
		Storage: st, Ledger: ld, Signals: signals,
		Orchestrator: core.NewOrchestrator(st, ld, runners, core.WithSignalCenter(signals),
			core.WithWorktreeProvisioner(simulateWorktrees{}), core.WithDossierCommit(false)),
	}
}

// simulateWorktrees is the --simulate root's WorktreeProvisioner: the walk's
// phases never write, so every "worktree" is the project root itself — no git
// worktree is added, no cycle-* branch is created, and cleanup is a no-op
// (never delete the operator's root). core recognises the in-place root
// (inPlaceWorktree) and stands its worktree mutators down. Deliberately no
// CreateFrom/reuse contract: continuation adoption is a no-op under simulate —
// a walk has no preserved work to adopt.
type simulateWorktrees struct{}

func (simulateWorktrees) Create(projectRoot string, _ int) (string, error) { return projectRoot, nil }
func (simulateWorktrees) Cleanup(_, _ string) error                        { return nil }
