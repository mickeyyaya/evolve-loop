package policy

import "path/filepath"

// WorkflowPolicy is the .evolve/policy.json "workflow" block.
type WorkflowPolicy struct {
	MaxConsecutiveFails   int               `json:"max_consecutive_fails,omitempty"`
	MaxCyclesCap          int               `json:"max_cycles_cap,omitempty"`
	AutoPrune             *bool             `json:"auto_prune,omitempty"`
	BackfillEnabled       *bool             `json:"backfill_enabled,omitempty"`
	CycleBudget           string            `json:"cycle_budget,omitempty"`
	AllowDeepResearch     bool              `json:"allow_deep_research,omitempty"`
	AllowDocDelete        bool              `json:"allow_doc_delete,omitempty"`
	DiffComplexityDisable bool              `json:"diff_complexity_disable,omitempty"`
	AuditorTierOverride   string            `json:"auditor_tier_override,omitempty"`
	PhaseEnables          map[string]string `json:"phase_enables,omitempty"`
	ConsensusAuditEnabled *bool             `json:"consensus_audit_enabled,omitempty"`
	// PSMASEnabled opts in to the Phase Scheduling and Management Advisor Subsystem.
	PSMASEnabled *bool `json:"psmas_enabled,omitempty"`
	// StrictAudit promotes a WARN audit verdict to FAIL in the audit phase and the
	// ship audit-binding, and makes the failure-adapter block. False is the default.
	StrictAudit bool `json:"strict_audit,omitempty"`
	// CompactPrompts strips "## Reference Index (Layer 3, on-demand)" and everything
	// after it from agent docs before dispatch; nil means on.
	CompactPrompts    *bool `json:"compact_prompts,omitempty"`
	UniversalFallback *bool `json:"universal_fallback,omitempty"`
	// UniversalFallbackExclude: absent means ["agy"]; an explicit [] lifts the ban.
	UniversalFallbackExclude []string `json:"universal_fallback_exclude,omitempty"`
	RemediationRounds        *int     `json:"remediation_rounds,omitempty"`
	// SizeBudgetMultipliers scale per-cycle budgets by cycle_size_estimate; each positive key overrides its default.
	// See ADR-0076.
	SizeBudgetMultipliers map[string]float64 `json:"size_budget_multipliers,omitempty"`
	RemediablePhases      []string           `json:"remediable_phases,omitempty"`
	BuildFloor            *bool              `json:"build_floor,omitempty"`
	InteractivePolicy     string             `json:"interactive_policy,omitempty"`
	InteractivePolicies   map[string]string  `json:"interactive_policies,omitempty"`
}

// WorkflowConfig is the resolved workflow configuration with defaults applied.
type WorkflowConfig struct {
	MaxConsecutiveFails   int
	MaxCyclesCap          int
	AutoPrune             bool
	BackfillEnabled       bool
	CycleBudget           string
	AllowDeepResearch     bool
	AllowDocDelete        bool
	DiffComplexityDisable bool
	AuditorTierOverride   string
	PhaseEnables          map[string]string
	ConsensusAuditEnabled bool
	PSMASEnabled          bool
	StrictAudit           bool
	CompactPrompts        bool
	// BuildFloorEnforced rejects the build deliverable while the changed packages'
	// self-check fails; false makes the self-check advisory.
	BuildFloorEnforced    bool
	SizeBudgetMultipliers map[string]float64
	// RemediationRounds is how many times a FAIL from a RemediablePhases gate
	// re-dispatches the builder with that gate's report, then re-runs the gate; 0 disables.
	RemediationRounds int
	// RemediablePhases must list only deterministic gates, never judgment phases such as audit.
	RemediablePhases []string
	// UniversalFallback appends the installed, phase-allowed CLIs to a launch chain
	// whose configured CLIs are all missing, instead of halting.
	UniversalFallback bool
	// UniversalFallbackExclude names families never used as the last-resort tail;
	// they may still be a profile's configured primary.
	UniversalFallbackExclude []string
	InteractivePolicy        string
	InteractivePolicies      map[string]string
}

// WorkflowConfig returns workflow configuration with built-in defaults resolved.
func (p Policy) WorkflowConfig() WorkflowConfig {
	c := WorkflowConfig{
		MaxConsecutiveFails: 1,
		MaxCyclesCap:        25,
		// "enforce" lets the advisor choose the cycle count when --cycles is omitted, capped by MaxCyclesCap.
		CycleBudget:              "enforce",
		AutoPrune:                true,
		BackfillEnabled:          true,
		ConsensusAuditEnabled:    true,
		CompactPrompts:           true,
		UniversalFallback:        true,
		UniversalFallbackExclude: []string{"agy"},
		RemediationRounds:        1,
		RemediablePhases:         []string{"coverage-gate"},
		BuildFloorEnforced:       true,
		SizeBudgetMultipliers:    map[string]float64{"trivial": 1.0, "small": 1.0, "medium": 1.25, "large": 1.5},
		InteractivePolicy:        "recommended_or_first",
	}
	if p.Workflow == nil {
		return c
	}
	if p.Workflow.MaxConsecutiveFails > 0 {
		c.MaxConsecutiveFails = p.Workflow.MaxConsecutiveFails
	}
	if p.Workflow.MaxCyclesCap > 0 {
		c.MaxCyclesCap = p.Workflow.MaxCyclesCap
	}
	if p.Workflow.AutoPrune != nil {
		c.AutoPrune = *p.Workflow.AutoPrune
	}
	if p.Workflow.BackfillEnabled != nil {
		c.BackfillEnabled = *p.Workflow.BackfillEnabled
	}
	if p.Workflow.CycleBudget != "" {
		c.CycleBudget = p.Workflow.CycleBudget
	}
	c.AllowDeepResearch = p.Workflow.AllowDeepResearch
	c.AllowDocDelete = p.Workflow.AllowDocDelete
	c.DiffComplexityDisable = p.Workflow.DiffComplexityDisable
	c.AuditorTierOverride = p.Workflow.AuditorTierOverride
	c.PhaseEnables = p.Workflow.PhaseEnables
	if p.Workflow.ConsensusAuditEnabled != nil {
		c.ConsensusAuditEnabled = *p.Workflow.ConsensusAuditEnabled
	}
	if p.Workflow.PSMASEnabled != nil {
		c.PSMASEnabled = *p.Workflow.PSMASEnabled
	}
	c.StrictAudit = p.Workflow.StrictAudit
	if p.Workflow.CompactPrompts != nil {
		c.CompactPrompts = *p.Workflow.CompactPrompts
	}
	if p.Workflow.UniversalFallback != nil {
		c.UniversalFallback = *p.Workflow.UniversalFallback
	}
	if p.Workflow.UniversalFallbackExclude != nil {
		c.UniversalFallbackExclude = append([]string(nil), p.Workflow.UniversalFallbackExclude...)
	}
	for k, v := range p.Workflow.SizeBudgetMultipliers {
		if v > 0 {
			c.SizeBudgetMultipliers[k] = v
		}
	}
	if p.Workflow.RemediationRounds != nil {
		c.RemediationRounds = *p.Workflow.RemediationRounds
	}
	if p.Workflow.RemediablePhases != nil {
		c.RemediablePhases = p.Workflow.RemediablePhases
	}
	if p.Workflow.BuildFloor != nil {
		c.BuildFloorEnforced = *p.Workflow.BuildFloor
	}
	if p.Workflow.InteractivePolicy != "" {
		c.InteractivePolicy = p.Workflow.InteractivePolicy
	}
	if p.Workflow.InteractivePolicies != nil {
		c.InteractivePolicies = make(map[string]string)
		for k, v := range p.Workflow.InteractivePolicies {
			c.InteractivePolicies[k] = v
		}
	}
	if p.Workflow.RemediationRounds != nil {
		c.RemediationRounds = *p.Workflow.RemediationRounds
	}
	if p.Workflow.RemediablePhases != nil {
		c.RemediablePhases = p.Workflow.RemediablePhases
	}
	if p.Workflow.BuildFloor != nil {
		c.BuildFloorEnforced = *p.Workflow.BuildFloor
	}
	return c
}

// StrictAuditFor loads projectRoot's policy and returns workflow.strict_audit, or false if the file is unreadable.
func StrictAuditFor(projectRoot string) bool {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return false
	}
	return pol.WorkflowConfig().StrictAudit
}

// InteractivePolicyFor returns agent's interactive policy: per-agent, else global, else "recommended_or_first".
func InteractivePolicyFor(projectRoot string, agent string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return "recommended_or_first"
	}
	cfg := pol.WorkflowConfig()
	if agent != "" && cfg.InteractivePolicies != nil {
		if v, ok := cfg.InteractivePolicies[agent]; ok && v != "" {
			return v
		}
	}
	return cfg.InteractivePolicy
}
