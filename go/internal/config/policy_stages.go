package config

// PolicyStages is the root's projection of the .evolve/policy.json dials. config cannot import
// policy, so the dials cross as plain values. Tests construct it positionally so a new field breaks the build.
type PolicyStages struct {
	// The gates.* keys, on the GateStage trichotomy.
	ContractGate, EvalGate, TriageCapGate, TopNGate, ReviewGate string
	// The recovery.* keys, on the GateStage trichotomy.
	PhaseRecovery, SpineFloor, FatalPane string
	// router.router_replan, on the RouterStage ladder.
	RouterReplan string
	// parallel_evaluate.stage, on the RouterStage ladder; policy sanitises it first, so it never warns here.
	ParallelEvaluate string
	// Copied as policy resolved them.
	ParallelEvaluateConcurrency int
	RoutingJudge, ReconDigest   bool
	RePlanMaxDepth              int
}

// ApplyPolicyStages returns cfg with the policy dials applied; a stage word outside its ladder
// resolves to off with a warning that names the policy key.
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
