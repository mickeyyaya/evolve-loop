package config

import "maps"

type PolicyStages struct {
	ContractGate, EvalGate, TriageCapGate, TopNGate, ReviewGate string
	PhaseRecovery, SpineFloor, FatalPane                        string
	RouterReplan                                                string
	ParallelEvaluate                                            string
	ParallelEvaluateConcurrency                                 int
	RoutingJudge, ReconDigest                                   bool
	RePlanMaxDepth                                              int
}

type RoutingKnobs struct {
	DynamicRouting, RoutingMode, PhaseIO string
	ConditionalMandatory                 map[string]string
	MaxOptionalInsertions                *int
}

func (l *Loader) ApplyPolicyStages(cfg RoutingConfig, ps PolicyStages) (RoutingConfig, []Warning) {
	var ws []Warning
	cfg.ContractGate = parseEvidenceStage(ps.ContractGate, "gates.contract_gate", &ws)
	cfg.EvalGate = parseEvidenceStage(ps.EvalGate, "gates.eval_gate", &ws)
	cfg.TriageCapGate = parseEvidenceStage(ps.TriageCapGate, "gates.triage_cap_gate", &ws)
	cfg.TopNGate = parseEvidenceStage(ps.TopNGate, "gates.topn_gate", &ws)
	cfg.ReviewGate = parseEvidenceStage(ps.ReviewGate, "gates.review_gate", &ws)
	cfg.PhaseRecovery = parseEvidenceStage(ps.PhaseRecovery, "recovery.phase_recovery", &ws)
	cfg.SpineFloor = parseEvidenceStage(ps.SpineFloor, "recovery.spine_floor", &ws)
	cfg.FatalPane = parseEvidenceStage(ps.FatalPane, "recovery.fatal_pane", &ws)
	cfg.RouterReplan = parseStage(ps.RouterReplan, "router.router_replan", &ws)
	cfg.ParallelEvaluate = parseStage(ps.ParallelEvaluate, "parallel_evaluate.stage", &ws)
	cfg.ParallelEvaluateConcurrency = ps.ParallelEvaluateConcurrency
	cfg.RoutingJudge = ps.RoutingJudge
	cfg.ReconDigest = ps.ReconDigest
	cfg.RePlanMaxDepth = ps.RePlanMaxDepth
	stamp(ws, "step", "policy", "source", "policy")
	l.emit("Loader.ApplyPolicyStages", ws)
	return cfg, ws
}

func (l *Loader) ApplyRoutingKnobs(cfg RoutingConfig, k RoutingKnobs) (RoutingConfig, []Warning) {
	var ws []Warning
	applyRoutingKnobs(&cfg, k, &ws)
	stamp(ws, "step", "policy", "source", "policy")
	l.emit("Loader.ApplyRoutingKnobs", ws)
	return cfg, ws
}

func applyRoutingKnobs(cfg *RoutingConfig, k RoutingKnobs, ws *[]Warning) {
	applyRegistryDials(cfg, registryConfig{DynamicRouting: k.DynamicRouting, RoutingMode: k.RoutingMode, MaxOptionalInsertions: k.MaxOptionalInsertions}, ws)
	if len(k.ConditionalMandatory) > 0 {
		cfg.Conditional = maps.Clone(cfg.Conditional)
		if cfg.Conditional == nil {
			cfg.Conditional = map[string]CondRule{}
		}
		applyConditional(cfg, k.ConditionalMandatory, ws)
	}
	if k.PhaseIO != "" {
		cfg.PhaseIO = parseStage(k.PhaseIO, "gates.phase_io", ws)
	}
	if k.DynamicRouting != "" {
		validateInertEnables(*cfg, ws)
	}
}
