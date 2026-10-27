package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func wiredRoutingConfigLoader(signals *signalcenter.Center) *config.Loader {
	return config.New(config.WithSignals(func() *signalcenter.Center { return signals }))
}

func policyStagesOf(g policy.GatesConfig, r policy.RecoveryPolicy, rt policy.RouterPolicy, pe policy.ParallelEvaluateConfig) config.PolicyStages {
	return config.PolicyStages{
		ContractGate: g.ContractGate, EvalGate: g.EvalGate, TriageCapGate: g.TriageCapGate, TopNGate: g.TopNGate, ReviewGate: g.ReviewGate,
		PhaseRecovery: r.PhaseRecovery, SpineFloor: r.SpineFloor, FatalPane: r.FatalPane,
		RouterReplan: rt.RouterReplan, ParallelEvaluate: pe.Stage, ParallelEvaluateConcurrency: pe.Concurrency,
		RoutingJudge: rt.RoutingJudge, ReconDigest: rt.ReconDigest, RePlanMaxDepth: rt.ReplanDepth,
	}
}

type bridgeStageSink interface {
	SetPhaseIOStage(config.Stage)
	SetRecoveryStage(string)
	SetFatalPaneStage(string)
}

func wireBridgeStages(br bridgeStageSink, cfg config.RoutingConfig) {
	br.SetPhaseIOStage(cfg.PhaseIO)
	br.SetRecoveryStage(cfg.PhaseRecovery.String())
	br.SetFatalPaneStage(cfg.FatalPane.String())
}

func productionBuildFloorChecks(ctx context.Context, in core.ReviewInput) []string {
	out := append(core.ProtectedSurfaceFloorChecks(guards.IsProtectedSurface)(ctx, in), core.DefaultBuildFloorChecks(ctx, in)...)
	out = append(out, core.ScopeDeltaFloorChecks(guards.IsProtectedSurface)(ctx, in)...)
	return append(out, core.RepoContractFloorChecks(repoContractPack)(ctx, in)...)
}

var repoContractPack core.RepoContractPackFn = ship.RunRepoContractPack

const (
	productionFloorCheckName = "production"
	solutionFloorCheckName   = "document-solution"
)

func composedBuildHandoffFloor(wf policy.WorkflowConfig, cfg config.RoutingConfig) core.BuildHandoffFloor {
	if !wf.BuildFloorEnforced {
		return nil
	}
	floor := core.BuildHandoffFloor{{Name: productionFloorCheckName, Run: productionBuildFloorChecks}}
	if spec, ok := cfg.DocumentSpec(); ok {
		floor = append(floor, core.BuildFloorCheck{Name: solutionFloorCheckName, Run: core.SolutionFloorChecks(spec)})
	}
	return floor
}

func probeBuildHandoffFloor(projectRoot string) core.BuildHandoffFloor {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[policy] WARN %v (the build handoff floor uses the compiled workflow defaults, as the cycle does)\n", err)
		pol = policy.Policy{}
	}
	cfg, _ := config.Load(config.RegistryPath(projectRoot))
	return core.WholeBuildHandoffFloor(composedBuildHandoffFloor(pol.WorkflowConfig(), cfg))
}

func parseGateStage(stage string) config.Stage {
	s, _ := config.GateStage(stage)
	return s
}

func parseRouterStage(stage string) config.Stage {
	s, _ := config.RouterStage(stage)
	return s
}

func warnRetiredControls(w io.Writer, environ []string) {
	for _, r := range ipcenv.RetiredInUse(environ) {
		fmt.Fprintf(w, "[policy] WARN %s is set, but the pipeline does not read it (ADR-0156 I-16): use %s\n", r.Name, r.Use)
	}
}
