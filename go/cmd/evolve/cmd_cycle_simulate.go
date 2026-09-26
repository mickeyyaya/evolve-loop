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

// simulatePhase is a core.PhaseRunner that returns PASS without calling out.
type simulatePhase struct {
	name core.Phase
}

func (s *simulatePhase) Name() string { return string(s.name) }

func (s *simulatePhase) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	// Write the contracted report stub, so the floors still grade a deliverable.
	if req.Workspace != "" {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, fmt.Errorf("simulate %s: workspace: %w", s.name, err)
		}
		// The sentinel and the declaration come from their owners, so the walk tracks
		// the grammar the real producers meet.
		stub := "# " + string(s.name) + " (simulate)\n\n" + phasecontract.RenderVerdictSentinel(string(s.name), core.VerdictPASS) + "\n"
		if s.name == core.PhaseBuild {
			// A walk produces no Build diff, so it declares explanation docs not applicable.
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

// simulatePhases covers every phase the canonical order or the core set can
// route to; extra runners are harmless. TestSimulatePhases_CoversCanonicalOrder
// guards it.
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

// wireSimulateOrchestrator builds the --simulate root: stub runners, the
// plane's storage and ledger, and the production signal topology, so warnings
// render as in a real cycle. It has no bridge: the walk launches no sessions.
func wireSimulateOrchestrator(projectRoot, evolveDir string, console io.Writer) orchDeps {
	phases := simulatePhases()
	runners := make(map[core.Phase]core.PhaseRunner, len(phases))
	for _, p := range phases {
		runners[p] = &simulatePhase{name: p}
	}

	signals := newRootSignalCenter(projectRoot, evolveDir, console)
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir, ledger.WithSignals(signals))
	// A walk never mutates the operator's repository: no cycle worktree or branch,
	// and no dossier closeout commit (the record is still written).
	return orchDeps{
		Storage: st, Ledger: ld, Signals: signals,
		Orchestrator: core.NewOrchestrator(st, ld, runners, core.WithSignalCenter(signals),
			core.WithWorktreeProvisioner(simulateWorktrees{}), core.WithDossierCommit(false)),
	}
}

// simulateWorktrees hands out the project root itself as every worktree, and
// its cleanup never deletes it. It has no CreateFrom: a walk has nothing to adopt.
type simulateWorktrees struct{}

func (simulateWorktrees) Create(projectRoot string, _ int) (string, error) { return projectRoot, nil }
func (simulateWorktrees) Cleanup(_, _ string) error                        { return nil }
