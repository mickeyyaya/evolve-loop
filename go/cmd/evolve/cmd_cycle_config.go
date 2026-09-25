package main

// cmd_cycle_config.go — the ADR-0103 unit-08 seam: the routing-config
// Loader's ONE wired construction (the root's Center is built 31 lines before
// the load, so the Loader takes it at construction — no lazy accessor pair),
// the ONE projection of policy's four accessors onto the leaf's Parameter
// Object, and the two silent stage forwarders the report-size gate
// (cmd_cycle.go) and the nested-sandbox fallback (cmd_loop_preflight.go)
// keep — their dials are not RoutingConfig fields yet (unit doc F2) — and
// (F27) the ONE forwarding of the resolved bridge dials into the adapter
// (wireBridgeStages), and (F37) the ONE composition of the build handoff
// floor (productionBuildFloorChecks) — on the protected manifest, so no cycle
// can drop the protected-surface check from its own floor.

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// wiredRoutingConfigLoader is the ONE construction of the Loader
// (TestRoutingConfigLoader_OneConstructionSite): every warning it resolves
// rides the root's Center as config.warning, which the root's StderrSink
// renders and the cycle-less durable sink files under <evolveDir>.
func wiredRoutingConfigLoader(signals *signalcenter.Center) *config.Loader {
	return config.New(config.WithSignals(func() *signalcenter.Center { return signals }))
}

// policyStagesOf is the ONE projection of policy's gate, recovery, router and
// parallel-evaluate accessors onto the leaf's PolicyStages
// (TestPolicyStagesOf_ProjectsThePolicyAccessors is the consumer pin).
func policyStagesOf(g policy.GatesConfig, r policy.RecoveryPolicy, rt policy.RouterPolicy, pe policy.ParallelEvaluateConfig) config.PolicyStages {
	return config.PolicyStages{
		ContractGate: g.ContractGate, EvalGate: g.EvalGate, TriageCapGate: g.TriageCapGate, TopNGate: g.TopNGate, ReviewGate: g.ReviewGate,
		PhaseRecovery: r.PhaseRecovery, SpineFloor: r.SpineFloor, FatalPane: r.FatalPane,
		RouterReplan: rt.RouterReplan, ParallelEvaluate: pe.Stage, ParallelEvaluateConcurrency: pe.Concurrency,
		RoutingJudge: rt.RoutingJudge, ReconDigest: rt.ReconDigest, RePlanMaxDepth: rt.ReplanDepth,
	}
}

// bridgeStageSink is the slice of the bridge adapter the root's resolved
// rollout dials flow into — the root depends on the three setters, not on the
// adapter, so the forwarding is testable without a live engine.
type bridgeStageSink interface {
	SetPhaseIOStage(config.Stage)
	SetRecoveryStage(string)
	SetFatalPaneStage(string)
}

// wireBridgeStages is the ONE forwarding of the resolved rollout dials into
// the bridge adapter (TestWireBridgeStages_IsTheRootsOnlyStageForwarding):
// each dial rides its OWN setter — the fatal-pane fast-fail never borrows
// PhaseRecovery's (F27; TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter).
func wireBridgeStages(br bridgeStageSink, cfg config.RoutingConfig) {
	br.SetPhaseIOStage(cfg.PhaseIO)
	br.SetRecoveryStage(cfg.PhaseRecovery.String())
	br.SetFatalPaneStage(cfg.FatalPane.String())
}

// productionBuildFloorChecks is the ONE composition of the code-cycle build
// handoff floor both roots run — the cycle reviewer (cmd_cycle.go) and the
// in-session pre-flight (`evolve selfcheck build`): the protected-surface floor
// with the control-plane membership predicate injected (F37 — core cannot
// import guards), judged FIRST, before go test can leave untracked files
// behind, then the deterministic engine. A named function, so the wiring pin's
// pointer identifies it (every ChainBuildFloorChecks closure shares one code
// pointer).
func productionBuildFloorChecks(ctx context.Context, in core.ReviewInput) []string {
	return append(core.ProtectedSurfaceFloorChecks(guards.IsProtectedSurface)(ctx, in), core.DefaultBuildFloorChecks(ctx, in)...)
}

// parseGateStage maps a policy gate word onto the off/shadow/enforce
// trichotomy, silently (an unknown word is off) — the mapping
// TestParseGateStage pins; the report-size gate (cmd_cycle.go, ReportSizeGate)
// and the nested-sandbox fallback (cmd_loop_preflight.go, NestedFallbackStage)
// read it. The RoutingConfig dials go through Loader.ApplyPolicyStages, which
// warns on the same ladder. DELIBERATELY still silent: the typo hole closes
// under unit doc F2 — make those two dials RoutingConfig/PolicyStages fields,
// then delete this forwarder — never by adding a warning here (it would print
// outside the Center, the line this unit removed).
func parseGateStage(stage string) config.Stage {
	s, _ := config.GateStage(stage)
	return s
}

// parseRouterStage maps a policy word onto the full off→shadow→advisory→enforce
// ladder, silently — kept beside its gate twin for the same readers and under
// the same F2 removal condition.
func parseRouterStage(stage string) config.Stage {
	s, _ := config.RouterStage(stage)
	return s
}
