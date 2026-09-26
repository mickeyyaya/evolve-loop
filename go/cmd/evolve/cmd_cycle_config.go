package main

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// wiredRoutingConfigLoader is the one construction of the Loader; its warnings
// ride the root's Center as config.warning.
func wiredRoutingConfigLoader(signals *signalcenter.Center) *config.Loader {
	return config.New(config.WithSignals(func() *signalcenter.Center { return signals }))
}

// policyStagesOf is the one projection of policy's gate, recovery, router and
// parallel-evaluate accessors onto config.PolicyStages.
func policyStagesOf(g policy.GatesConfig, r policy.RecoveryPolicy, rt policy.RouterPolicy, pe policy.ParallelEvaluateConfig) config.PolicyStages {
	return config.PolicyStages{
		ContractGate: g.ContractGate, EvalGate: g.EvalGate, TriageCapGate: g.TriageCapGate, TopNGate: g.TopNGate, ReviewGate: g.ReviewGate,
		PhaseRecovery: r.PhaseRecovery, SpineFloor: r.SpineFloor, FatalPane: r.FatalPane,
		RouterReplan: rt.RouterReplan, ParallelEvaluate: pe.Stage, ParallelEvaluateConcurrency: pe.Concurrency,
		RoutingJudge: rt.RoutingJudge, ReconDigest: rt.ReconDigest, RePlanMaxDepth: rt.ReplanDepth,
	}
}

// bridgeStageSink is the slice of the bridge adapter the root's rollout dials
// flow into, so the forwarding is testable without a live engine.
type bridgeStageSink interface {
	SetPhaseIOStage(config.Stage)
	SetRecoveryStage(string)
	SetFatalPaneStage(string)
}

// wireBridgeStages is the one forwarding of the resolved rollout dials into
// the bridge adapter; each dial rides its own setter.
func wireBridgeStages(br bridgeStageSink, cfg config.RoutingConfig) {
	br.SetPhaseIOStage(cfg.PhaseIO)
	br.SetRecoveryStage(cfg.PhaseRecovery.String())
	br.SetFatalPaneStage(cfg.FatalPane.String())
}

// productionBuildFloorChecks is the one build handoff floor that the cycle
// reviewer and `evolve selfcheck build` both run. The protected-surface floor
// runs first, before go test can leave untracked files behind. A named func,
// so the wiring pin's pointer identifies it.
func productionBuildFloorChecks(ctx context.Context, in core.ReviewInput) []string {
	return append(core.ProtectedSurfaceFloorChecks(guards.IsProtectedSurface)(ctx, in), core.DefaultBuildFloorChecks(ctx, in)...)
}

// parseGateStage maps a gate word onto off/shadow/enforce, silently: an unknown
// word is off. It stays silent until its two readers' dials become PolicyStages
// fields; a warning here would print outside the Center.
func parseGateStage(stage string) config.Stage {
	s, _ := config.GateStage(stage)
	return s
}

// parseRouterStage maps a word onto off/shadow/advisory/enforce, silently, for
// the same readers and until the same removal as parseGateStage.
func parseRouterStage(stage string) config.Stage {
	s, _ := config.RouterStage(stage)
	return s
}
