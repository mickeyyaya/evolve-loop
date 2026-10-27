package policy

import (
	"maps"
	"path/filepath"
	"slices"
)

type WorkflowPolicy struct {
	MaxConsecutiveFails      int                   `json:"max_consecutive_fails,omitempty"`
	MaxCyclesCap             int                   `json:"max_cycles_cap,omitempty"`
	AutoPrune                *bool                 `json:"auto_prune,omitempty"`
	BackfillEnabled          *bool                 `json:"backfill_enabled,omitempty"`
	CycleBudget              string                `json:"cycle_budget,omitempty"`
	AllowDocDelete           bool                  `json:"allow_doc_delete,omitempty"`
	DiffComplexityDisable    bool                  `json:"diff_complexity_disable,omitempty"`
	AuditorTierOverride      string                `json:"auditor_tier_override,omitempty"`
	PhaseEnables             map[string]string     `json:"phase_enables,omitempty"`
	ConsensusAuditEnabled    *bool                 `json:"consensus_audit_enabled,omitempty"`
	PSMASEnabled             *bool                 `json:"psmas_enabled,omitempty"`
	StrictAudit              bool                  `json:"strict_audit,omitempty"`
	CompactPrompts           *bool                 `json:"compact_prompts,omitempty"`
	UniversalFallback        *bool                 `json:"universal_fallback,omitempty"`
	UniversalFallbackExclude []string              `json:"universal_fallback_exclude,omitempty"`
	RemediationRounds        *int                  `json:"remediation_rounds,omitempty"`
	SizeBudgetMultipliers    map[string]float64    `json:"size_budget_multipliers,omitempty"`
	RemediablePhases         []string              `json:"remediable_phases,omitempty"`
	BuildFloor               *bool                 `json:"build_floor,omitempty"`
	InteractivePolicy        string                `json:"interactive_policy,omitempty"`
	InteractivePolicies      map[string]string     `json:"interactive_policies,omitempty"`
	FindingsRepair           *FindingsRepairPolicy `json:"findings_repair,omitempty"`
	QualityIndex             *QualityIndexPolicy   `json:"quality_index,omitempty"`
	Convergence              *ConvergencePolicy    `json:"convergence,omitempty"`
	RetryLadder              *RetryLadderPolicy    `json:"retry_ladder,omitempty"`
	Repair                   *RepairPolicy         `json:"repair,omitempty"`
	IntentDelta              bool                  `json:"intent_delta,omitempty"`
}

type WorkflowConfig struct {
	MaxConsecutiveFails      int
	MaxCyclesCap             int
	AutoPrune                bool
	BackfillEnabled          bool
	CycleBudget              string
	AllowDocDelete           bool
	DiffComplexityDisable    bool
	AuditorTierOverride      string
	PhaseEnables             map[string]string
	ConsensusAuditEnabled    bool
	PSMASEnabled             bool
	StrictAudit              bool
	CompactPrompts           bool
	BuildFloorEnforced       bool
	SizeBudgetMultipliers    map[string]float64
	RemediationRounds        int
	RemediablePhases         []string
	UniversalFallback        bool
	UniversalFallbackExclude []string
	InteractivePolicy        string
	InteractivePolicies      map[string]string
	CodeReviewRepair         JudgeRepairConfig
	QualityIndex             QualityIndexConfig
	RetryLadder              RetryLadderConfig
	Repair                   RepairConfig
	IntentDelta              bool
}

func (p Policy) WorkflowConfig() WorkflowConfig {
	c := p.defaultWorkflowConfig()
	if p.Workflow == nil {
		return c
	}
	w := *p.Workflow
	for _, section := range []func(WorkflowConfig) WorkflowConfig{w.withLoopLimits, w.withPhaseSwitches, w.withFallback, w.withRemediation, w.withInteractive} {
		c = section(c)
	}
	return c
}

func (p Policy) defaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		MaxConsecutiveFails:      1,
		MaxCyclesCap:             25,
		CycleBudget:              "enforce",
		AutoPrune:                true,
		BackfillEnabled:          true,
		ConsensusAuditEnabled:    true,
		CompactPrompts:           true,
		UniversalFallback:        true,
		UniversalFallbackExclude: defaultUniversalFallbackExclude(),
		RemediationRounds:        1,
		RemediablePhases:         []string{"coverage-gate"},
		BuildFloorEnforced:       true,
		SizeBudgetMultipliers:    map[string]float64{"trivial": 1.0, "small": 1.0, "medium": 1.25, "large": 1.5},
		InteractivePolicy:        "recommended_or_first",
		CodeReviewRepair:         resolveJudgeRepair("code-review", p.codeReviewRepairPolicy()),
		QualityIndex:             p.qualityIndexConfig(),
		RetryLadder:              p.retryLadderConfig(),
		Repair:                   p.repairConfig(),
	}
}

func (w WorkflowPolicy) withLoopLimits(c WorkflowConfig) WorkflowConfig {
	if w.MaxConsecutiveFails > 0 {
		c.MaxConsecutiveFails = w.MaxConsecutiveFails
	}
	if w.MaxCyclesCap > 0 {
		c.MaxCyclesCap = w.MaxCyclesCap
	}
	if w.AutoPrune != nil {
		c.AutoPrune = *w.AutoPrune
	}
	if w.BackfillEnabled != nil {
		c.BackfillEnabled = *w.BackfillEnabled
	}
	if w.CycleBudget != "" {
		c.CycleBudget = w.CycleBudget
	}
	return c
}

func (w WorkflowPolicy) withPhaseSwitches(c WorkflowConfig) WorkflowConfig {
	c.AllowDocDelete = w.AllowDocDelete
	c.IntentDelta = w.IntentDelta
	c.DiffComplexityDisable = w.DiffComplexityDisable
	c.AuditorTierOverride = w.AuditorTierOverride
	c.PhaseEnables = maps.Clone(w.PhaseEnables)
	if w.ConsensusAuditEnabled != nil {
		c.ConsensusAuditEnabled = *w.ConsensusAuditEnabled
	}
	if w.PSMASEnabled != nil {
		c.PSMASEnabled = *w.PSMASEnabled
	}
	c.StrictAudit = w.StrictAudit
	if w.CompactPrompts != nil {
		c.CompactPrompts = *w.CompactPrompts
	}
	return c
}

func (w WorkflowPolicy) withFallback(c WorkflowConfig) WorkflowConfig {
	if w.UniversalFallback != nil {
		c.UniversalFallback = *w.UniversalFallback
	}
	if w.UniversalFallbackExclude != nil {
		c.UniversalFallbackExclude = append([]string(nil), w.UniversalFallbackExclude...)
	}
	return c
}

func (w WorkflowPolicy) withRemediation(c WorkflowConfig) WorkflowConfig {
	multipliers := maps.Clone(c.SizeBudgetMultipliers)
	for k, v := range w.SizeBudgetMultipliers {
		if v > 0 {
			multipliers[k] = v
		}
	}
	c.SizeBudgetMultipliers = multipliers
	if w.RemediationRounds != nil {
		c.RemediationRounds = *w.RemediationRounds
	}
	if w.RemediablePhases != nil {
		c.RemediablePhases = slices.Clone(w.RemediablePhases)
	}
	if w.BuildFloor != nil {
		c.BuildFloorEnforced = *w.BuildFloor
	}
	return c
}

func (w WorkflowPolicy) withInteractive(c WorkflowConfig) WorkflowConfig {
	if w.InteractivePolicy != "" {
		c.InteractivePolicy = w.InteractivePolicy
	}
	if w.InteractivePolicies != nil {
		c.InteractivePolicies = maps.Clone(w.InteractivePolicies)
	}
	return c
}

func StrictAuditFor(projectRoot string) bool {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return false
	}
	return pol.WorkflowConfig().StrictAudit
}

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
		if v, ok := legacyLabelValue(cfg.InteractivePolicies, "interactive_policies", agent, func(v string) bool { return v != "" }); ok {
			return v
		}
	}
	return cfg.InteractivePolicy
}
