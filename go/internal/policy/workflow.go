package policy

import "path/filepath"

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
	// PSMASEnabled enables the Phase Scheduling and Management Advisor
	// Subsystem. Absent/false = disabled (opt-in). Replaces EVOLVE_PSMAS_SKIP.
	PSMASEnabled *bool `json:"psmas_enabled,omitempty"`
	// StrictAudit selects the strict (legacy-blocking) audit posture. Absent/false
	// = fluent-by-default (ship on a WARN audit verdict; the failure-adapter is
	// awareness-only on recurring failures). True restores legacy blocking: WARN is
	// promoted to FAIL in both the audit phase and the ship audit-binding, and the
	// failure-adapter's first matching rule BLOCKs. Replaces the EVOLVE_STRICT_AUDIT
	// env read (flag-reduction, ADR-0064). A plain bool (not *bool): false is the
	// product default, so an absent block and an explicit false are the same posture.
	StrictAudit bool `json:"strict_audit,omitempty"`
	// CompactPrompts enables on-demand reference-section stripping from disk-loaded
	// agent docs before dispatch (strips "## Reference Index (Layer 3, on-demand)"
	// and everything after it). Absent/nil = default ON; explicit false opts out.
	CompactPrompts    *bool `json:"compact_prompts,omitempty"`
	UniversalFallback *bool `json:"universal_fallback,omitempty"`
	// RemediationRounds/RemediablePhases configure graduated remediation
	// (workflow.remediation_rounds / workflow.remediable_phases).
	RemediationRounds *int `json:"remediation_rounds,omitempty"`
	// SizeBudgetMultipliers scales per-cycle budgets (correction rounds, build
	// artifact timeout) by the triage/scout cycle_size_estimate (ADR-0076 A).
	// Per-key positive override; unmentioned keys keep compiled defaults.
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
	// CompactPrompts mirrors WorkflowPolicy.CompactPrompts with the default applied.
	// Default true: phase runners strip the on-demand reference tail before dispatch.
	CompactPrompts bool
	// BuildFloorEnforced (default true): the build deliverable is REJECTED
	// while the changed packages' deterministic self-check fails (shift-left
	// half of the 2026-07-21 directive) — the E2 correction ladder then fixes
	// it in-phase. false restores the advisory-only selfcheck.
	BuildFloorEnforced bool
	// SizeBudgetMultipliers maps cycle_size_estimate → budget multiplier
	// (ADR-0076 A). Compiled defaults: trivial/small 1.0, medium 1.25,
	// large 1.5. A survivorship-hard backlog starves the verification tail
	// under uniform budgets (batch-8: ~10-minute build windows incl.
	// corrections on structural items).
	SizeBudgetMultipliers map[string]float64
	// RemediationRounds bounds the graduated fix-forward ladder (operator
	// directive 2026-07-21): when a phase listed in RemediablePhases returns a
	// FAIL verdict, the orchestrator re-dispatches the builder ONCE per round
	// with the gate's report as a correction directive, then re-runs the SAME
	// gate. 0 disables. Default 1.
	RemediationRounds int
	// RemediablePhases lists the DETERMINISTIC gate phases eligible for
	// graduated remediation. Judgment phases (audit, adversarial-review,
	// premise-challenge) must never be listed — remediation is for mechanical,
	// prescribed defects only. Default ["coverage-gate"].
	RemediablePhases []string
	// UniversalFallback (default true): when a phase's whole configured CLI chain
	// (primary + cli_fallback) has no binary on this host, the runner discovers
	// installed+authed CLIs via bridge.Doctor and appends the phase-allowlisted
	// ones to the chain instead of halting (any_cli_any_phase invariant). Set
	// workflow.universal_fallback=false to hard-pin to the configured chain.
	UniversalFallback   bool
	InteractivePolicy   string
	InteractivePolicies map[string]string
}

// WorkflowConfig returns workflow configuration with built-in defaults resolved.
func (p Policy) WorkflowConfig() WorkflowConfig {
	c := WorkflowConfig{
		MaxConsecutiveFails: 1,
		MaxCyclesCap:        25,
		// Cycle count is optional: with no explicit --cycles the advisor decides
		// how many cycles the goal needs — completion-driven (stop when the
		// backlog drains), bounded by MaxCyclesCap. Override with
		// workflow.cycle_budget="off" in policy.json to restore a fixed count.
		CycleBudget:           "enforce",
		AutoPrune:             true,
		BackfillEnabled:       true,
		ConsensusAuditEnabled: true,
		CompactPrompts:        true, // default ON: strips ~23 KB/cycle of reference tails
		UniversalFallback:     true, // default ON: discover+route to an installed CLI when the configured chain is absent
		// Graduated remediation (2026-07-21): default ON at 1 round for the
		// coverage gate — the measured waste class (983/992/1007/1019/1020).
		RemediationRounds:     1,
		RemediablePhases:      []string{"coverage-gate"},
		BuildFloorEnforced:    true,
		SizeBudgetMultipliers: map[string]float64{"trivial": 1.0, "small": 1.0, "medium": 1.25, "large": 1.5},
		InteractivePolicy:     "recommended_or_first",
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

// StrictAuditFor loads the policy at projectRoot's .evolve/policy.json and returns
// the resolved workflow.strict_audit posture. Fail-open: a missing OR malformed
// policy yields false (fluent default) so a typo can never silently ARM the opt-in
// strict tightening — the loud malformed-policy failure still surfaces at the
// cycle's own policy.Load. The audit phase and the ship audit-binding both read
// strict mode from here (they have projectRoot but not the orchestrator's
// once-resolved WorkflowConfig), mirroring WorktreeBaseFor's loader pattern.
func StrictAuditFor(projectRoot string) bool {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return false
	}
	return pol.WorkflowConfig().StrictAudit
}

// InteractivePolicyFor returns the interactive policy for the specified agent.
// It loads the policy.json from the project root and returns the per-agent
// override if configured, falling back to the global policy, and finally
// defaulting to "recommended_or_first".
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

// RetryPolicy is the .evolve/policy.json "retry" block.
