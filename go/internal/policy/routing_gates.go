package policy

const defaultObservationMaskWindow = 10

// ObservationMaskPolicy is the .evolve/policy.json "observation_mask" block and
// its own resolved-config type (mirrors the RouterPolicy raw==resolved idiom):
// a single WindowTurns knob. It also serves as ObservationMaskConfig's return
// value with defaults applied.
type ObservationMaskPolicy struct {
	// WindowTurns is how many of the newest evictable tool observations stay
	// unmasked. Zero/negative/absent ⇒ defaultObservationMaskWindow (10). A
	// resolved WindowTurns<=0 is never produced by the getter; when a caller
	// passes it straight to phasestream.MaskStaleObservations, <=0 means
	// "feature off" (byte-identical passthrough).
	WindowTurns int `json:"window_turns,omitempty"`
}

// ObservationMaskConfig returns the observation-mask window with the built-in
// default resolved. An absent block, or a non-positive window_turns override,
// yields WindowTurns=10; a positive override passes through. No I/O, no env.
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

// RouterConfig returns router configuration with built-in defaults resolved.
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

// GatesPolicy is the .evolve/policy.json "gates" block.
type GatesPolicy struct {
	ContractGate  string `json:"contract_gate,omitempty"`
	EvalGate      string `json:"eval_gate,omitempty"`
	TriageCapGate string `json:"triage_cap_gate,omitempty"`
	ReviewGate    string `json:"review_gate,omitempty"`
	// TopNGate is the build->audit top_n task-binding gate's rollout dial
	// (internal/topngate). Default "enforce": a build report whose ## Task:
	// slug falls outside triage ## top_n aborts before audit.
	TopNGate string `json:"topn_gate,omitempty"`
	// ReportSizeGate is the report-size (handoff-summary token budget) gate's own
	// rollout dial (cycle-565 Slice S1). Unlike the other gates it defaults to
	// "shadow", not "enforce": the inbox spec calls for shadow/warn BEFORE
	// enforce so the budget is observed before it can block a cycle.
	ReportSizeGate string `json:"report_size_gate,omitempty"`
	// ManifestGate is the ship-bind tree-manifest reconciliation gate's rollout
	// dial (internal/phases/ship/manifest.go, cycle-1064). Like ReportSizeGate it
	// defaults to "shadow", not "enforce": out-of-manifest paths (the cross-lane
	// untracked-leak shape) are LOGGED before the gate is allowed to block a
	// cycle. "enforce" fails the ship closed with core.CodeManifestGate.
	ManifestGate string `json:"manifest_gate,omitempty"`

	// RepoContractGate is the ship-time repo-contract scanner pack's dial
	// (internal/phases/ship/repocontract.go). Unlike ManifestGate it defaults
	// to "enforce": the scanners are the deterministic repo-wide guard suites
	// (phasespec, profiles, phasecoherence, routingtest) whose breakage IS a
	// red main — four lane landings redded main in the week of 2026-08-04
	// (artifact-bytes, phase metadata, profile stubs, incident-postmortem),
	// each a CI-email storm; FP≈0 because a failing scanner here fails on
	// main's next run by construction. "off" disables.
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
		ContractGate:   "enforce",
		EvalGate:       "enforce",
		TriageCapGate:  "enforce",
		ReviewGate:     "off",
		ReportSizeGate: "shadow", // shadow/warn first, per the Slice S1 inbox spec
		TopNGate:       "enforce",
		ManifestGate:   "shadow", // shadow-first, mirroring ReportSizeGate (cycle-1064)
		// enforce-default deviation from shadow-first, justified: the pack is
		// existing deterministic repo tests (FP≈0), and every failure mode it
		// gates was a LIVE red-main incident in the preceding week.
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

// ReportBudgetPolicy is the .evolve/policy.json "report_budget" block: the
// per-artifact token budgets the report-size gate enforces (cycle-565 Slice S1).
// Separate from GatesPolicy so the budget VALUE and the gate STAGE are dialed
// independently. Absent ⇒ built-in defaults apply.
