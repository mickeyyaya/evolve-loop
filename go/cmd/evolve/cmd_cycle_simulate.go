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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type simulatePhase struct {
	name core.Phase
}

func (s *simulatePhase) Name() string { return string(s.name) }

func (s *simulatePhase) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if req.Workspace != "" {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, fmt.Errorf("simulate %s: workspace: %w", s.name, err)
		}
		stub := "# " + string(s.name) + " (simulate)\n\n" + phasecontract.RenderVerdictSentinel(string(s.name), core.VerdictPASS) + "\n"
		if s.name == core.PhaseBuild {
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

func wireSimulateOrchestrator(projectRoot, evolveDir string, console io.Writer) orchDeps {
	phases := simulatePhases()
	runners := make(map[core.Phase]core.PhaseRunner, len(phases))
	for _, p := range phases {
		runners[p] = &simulatePhase{name: p}
	}

	signals, closeSignals := newRoleSignalCenter(roleSimulate, projectRoot, evolveDir, console)
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir, ledger.WithSignals(signals))
	return orchDeps{
		Storage: st, Ledger: ld, Signals: signals, CloseSignals: closeSignals,
		Orchestrator: core.NewOrchestrator(st, ld, runners, core.WithSignalCenter(signals),
			core.WithWorktreeProvisioner(simulateWorktrees{}), core.WithDossierDestination(core.DossierFilesOnly),
			core.WithLaneMenu(laneMenuOf(laneForbiddenTo(projectRoot, signals.Emit), policy.FixabilityFor(projectRoot, "simulate", os.Stderr)))),
	}
}

type simulateWorktrees struct{}

func (simulateWorktrees) Create(projectRoot string, _ int) (string, error) { return projectRoot, nil }
func (simulateWorktrees) Cleanup(_, _ string) error                        { return nil }
