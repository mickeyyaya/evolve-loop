package config

// policy_stages.go — the second resolution step: the composition root's
// projection of .evolve/policy.json's gate, recovery, router and
// parallel-evaluate dials onto the resolved value. config can never import
// internal/policy (policy imports config), so the dials cross as plain
// strings, ints and bools in a Parameter Object; the two stage ladders are
// the same GateStage / RouterStage the loader's own parsers use, so a typo'd
// policy word resolves to off WITH a warning instead of the silent off the
// root's hand copies produced.

// PolicyStages is the strings/ints/bools Parameter Object the composition
// root projects from policy.GatesConfig / RecoveryConfig / RouterConfig /
// ParallelEvaluateConfig. Field order is the root's assignment order; the
// in-package tests construct it POSITIONALLY so a new field breaks the build.
type PolicyStages struct {
	// gates.contract_gate, gates.eval_gate, gates.triage_cap_gate,
	// gates.topn_gate, gates.review_gate — the GateStage trichotomy.
	ContractGate, EvalGate, TriageCapGate, TopNGate, ReviewGate string
	// recovery.phase_recovery, recovery.spine_floor — the GateStage trichotomy.
	PhaseRecovery, SpineFloor string
	// router.router_replan — the RouterStage ladder.
	RouterReplan string
	// parallel_evaluate.stage — the RouterStage ladder (policy sanitises the
	// word before it reaches the loader, so a typo never warns from this path).
	ParallelEvaluate string
	// parallel_evaluate.concurrency, router.routing_judge, router.recon_digest,
	// router.replan_depth — copied as resolved by policy.
	ParallelEvaluateConcurrency int
	RoutingJudge, ReconDigest   bool
	RePlanMaxDepth              int
}

// ApplyPolicyStages resolves the thirteen policy dials over cfg — cfg by
// VALUE in, a new value out. The seven gate/recovery dials use the
// off/shadow/enforce trichotomy, the two router dials the full ladder; a
// word outside its ladder resolves to off with a CONFIG_UNKNOWN_VALUE naming
// the policy key (fields.key = gates.eval_gate …, step = source = policy).
// Origin Loader.ApplyPolicyStages.
func (l *Loader) ApplyPolicyStages(cfg RoutingConfig, ps PolicyStages) (RoutingConfig, []Warning) {
	var ws []Warning
	cfg.ContractGate = parseEvidenceStage(ps.ContractGate, "gates.contract_gate", &ws)
	cfg.EvalGate = parseEvidenceStage(ps.EvalGate, "gates.eval_gate", &ws)
	cfg.TriageCapGate = parseEvidenceStage(ps.TriageCapGate, "gates.triage_cap_gate", &ws)
	cfg.TopNGate = parseEvidenceStage(ps.TopNGate, "gates.topn_gate", &ws)
	cfg.ReviewGate = parseEvidenceStage(ps.ReviewGate, "gates.review_gate", &ws)
	cfg.PhaseRecovery = parseEvidenceStage(ps.PhaseRecovery, "recovery.phase_recovery", &ws)
	cfg.SpineFloor = parseEvidenceStage(ps.SpineFloor, "recovery.spine_floor", &ws)
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
