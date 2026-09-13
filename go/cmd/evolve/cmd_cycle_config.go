package main

// cmd_cycle_config.go — the ADR-0103 unit-08 seam: the routing-config
// Loader's ONE wired construction (the root's Center is built 31 lines before
// the load, so the Loader takes it at construction — no lazy accessor pair),
// the ONE projection of policy's four accessors onto the leaf's Parameter
// Object, and the two silent stage forwarders the report-size gate
// (cmd_cycle.go) and the nested-sandbox fallback (cmd_loop_preflight.go)
// keep — their dials are not RoutingConfig fields yet (unit doc F2).

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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
		PhaseRecovery: r.PhaseRecovery, SpineFloor: r.SpineFloor,
		RouterReplan: rt.RouterReplan, ParallelEvaluate: pe.Stage, ParallelEvaluateConcurrency: pe.Concurrency,
		RoutingJudge: rt.RoutingJudge, ReconDigest: rt.ReconDigest, RePlanMaxDepth: rt.ReplanDepth,
	}
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
