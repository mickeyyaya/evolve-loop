package policy

// defaultObservationMaskWindow is the empirical optimum (M=10) from the
// observation-masking study, arXiv 2508.21433.
const defaultObservationMaskWindow = 10

// ObservationMaskPolicy is the "observation_mask" block and also its own resolved value.
type ObservationMaskPolicy struct {
	// WindowTurns is how many newest evictable tool observations stay unmasked;
	// non-positive means 10. The getter never returns <=0, which phasestream reads as off.
	WindowTurns int `json:"window_turns,omitempty"`
}

// ObservationMaskConfig returns the observation-mask window, defaulting to 10.
func (p Policy) ObservationMaskConfig() ObservationMaskPolicy {
	c := ObservationMaskPolicy{WindowTurns: defaultObservationMaskWindow}
	if p.ObservationMask != nil && p.ObservationMask.WindowTurns > 0 {
		c.WindowTurns = p.ObservationMask.WindowTurns
	}
	return c
}

// RouterPolicy is the .evolve/policy.json "router" block.
type RouterPolicy struct {
	RouterReplan string `json:"router_replan,omitempty"`
	RoutingJudge bool   `json:"routing_judge,omitempty"`
	ReconDigest  bool   `json:"recon_digest,omitempty"`
	ReplanDepth  int    `json:"replan_depth,omitempty"`
	PlanModel    string `json:"plan_model,omitempty"`
	ProposeModel string `json:"propose_model,omitempty"`
	CLI          string `json:"cli,omitempty"`
	Model        string `json:"model,omitempty"`
}

// RouterConfig returns router configuration, defaulting RouterReplan to "shadow" and ReplanDepth to 1.
func (p Policy) RouterConfig() RouterPolicy {
	c := RouterPolicy{RouterReplan: "shadow", ReplanDepth: 1}
	if p.Router == nil {
		return c
	}
	if p.Router.RouterReplan != "" {
		c.RouterReplan = p.Router.RouterReplan
	}
	c.RoutingJudge = p.Router.RoutingJudge
	c.ReconDigest = p.Router.ReconDigest
	if p.Router.ReplanDepth > 0 {
		c.ReplanDepth = p.Router.ReplanDepth
	}
	c.PlanModel = p.Router.PlanModel
	c.ProposeModel = p.Router.ProposeModel
	c.CLI = p.Router.CLI
	c.Model = p.Router.Model
	return c
}

// GatesPolicy is the .evolve/policy.json "gates" block of per-gate rollout stages.
type GatesPolicy struct {
	ContractGate  string `json:"contract_gate,omitempty"`
	EvalGate      string `json:"eval_gate,omitempty"`
	TriageCapGate string `json:"triage_cap_gate,omitempty"`
	ReviewGate    string `json:"review_gate,omitempty"`
	// TopNGate aborts before audit when the build report's task slug is outside triage top_n.
	TopNGate string `json:"topn_gate,omitempty"`
	// ReportSizeGate defaults to "shadow" so the handoff budget is observed before it can block.
	ReportSizeGate string `json:"report_size_gate,omitempty"`
	// ManifestGate defaults to "shadow": out-of-manifest ship paths are logged before
	// the gate may fail a ship with core.CodeManifestGate.
	ManifestGate string `json:"manifest_gate,omitempty"`

	// RepoContractGate runs the repo-wide guard suites at ship time; it defaults to
	// "enforce" because those suites fail on main's next run anyway (false-positive rate near zero).
	RepoContractGate string `json:"repo_contract_gate,omitempty"`
}

// GatesConfig is the resolved gate configuration with defaults applied.
type GatesConfig struct {
	ContractGate     string
	EvalGate         string
	TriageCapGate    string
	ReviewGate       string
	ReportSizeGate   string
	TopNGate         string
	ManifestGate     string
	RepoContractGate string
}

// GatesConfig returns persistent gate stages with built-in defaults resolved.
func (p Policy) GatesConfig() GatesConfig {
	c := GatesConfig{
		ContractGate:     "enforce",
		EvalGate:         "enforce",
		TriageCapGate:    "enforce",
		ReviewGate:       "off",
		ReportSizeGate:   "shadow",
		TopNGate:         "enforce",
		ManifestGate:     "shadow",
		RepoContractGate: "enforce",
	}
	if p.Gates == nil {
		return c
	}
	if p.Gates.ContractGate != "" {
		c.ContractGate = p.Gates.ContractGate
	}
	if p.Gates.EvalGate != "" {
		c.EvalGate = p.Gates.EvalGate
	}
	if p.Gates.TriageCapGate != "" {
		c.TriageCapGate = p.Gates.TriageCapGate
	}
	if p.Gates.ReviewGate != "" {
		c.ReviewGate = p.Gates.ReviewGate
	}
	if p.Gates.ReportSizeGate != "" {
		c.ReportSizeGate = p.Gates.ReportSizeGate
	}
	if p.Gates.TopNGate != "" {
		c.TopNGate = p.Gates.TopNGate
	}
	if p.Gates.ManifestGate != "" {
		c.ManifestGate = p.Gates.ManifestGate
	}
	if p.Gates.RepoContractGate != "" {
		c.RepoContractGate = p.Gates.RepoContractGate
	}
	return c
}
