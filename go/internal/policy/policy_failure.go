package policy

import "fmt"

// ADR-0072 — the system-failure DECISION policy. This is the declarative
// surface the orchestrator classifies each failure against; Go enforces the
// floor (verdict-incoherence, infra-systemic ALWAYS halt) and provides the
// deterministic fallback. See docs/architecture/adr/0072-system-failure-policy-and-halt.md.
//
// NOTE: the type is SystemFailurePolicy (not FailurePolicy) because
// (Policy).FailurePolicy() already resolves the distinct failure_floor
// LLM-learning block — this is the failure DECISION policy, a separate surface.

// Category level and action vocabularies. Kept as string consts (not an enum)
// to mirror the checked-in policy.json shape; validated in FailurePolicyConfig.
const (
	LevelSystem = "system"
	LevelTask   = "task"

	ActionHaltAndDiagnose   = "halt-and-diagnose"
	ActionRetryWithFix      = "retry-with-fix"
	ActionDeferOrQuarantine = "defer-or-quarantine"
)

// Floor category keys are the two non-negotiable halts: a broken pipeline
// cannot be talked out of stopping on these regardless of operator policy or
// orchestrator judgment (authority model: "orchestrator decides, Go enforces
// floor"). These keys are the policy vocabulary; the dossier layer maps the
// deterministic failureadapter.Classification onto them.
const (
	CategoryVerdictIncoherence = "verdict-incoherence"
	CategoryInfraSystemic      = "infra-systemic"
	CategoryTransportHang      = "transport-hang"
	CategoryNonProgress        = "non-progress"
	CategoryCodeBuildFail      = "code-build-fail"
	CategoryCodeAuditFail      = "code-audit-fail"
	CategoryIntentMalformed    = "intent-malformed"
)

// FailureCategory is one row of the failure_policy category table.
type FailureCategory struct {
	Level      string `json:"level"`                 // "system" | "task"
	Action     string `json:"action"`                // halt-and-diagnose | retry-with-fix | defer-or-quarantine
	FixType    string `json:"fix_type,omitempty"`    // the kind of fix the next cycle should deploy
	Signature  string `json:"signature,omitempty"`   // human-readable detection signature
	Floor      bool   `json:"floor,omitempty"`       // Go always-enforces halt (non-negotiable)
	MaxRetries int    `json:"max_retries,omitempty"` // task-level: retries before quarantine
}

// FailureThresholds are the non-progress / retry counters (ADR-0072 S2/S5).
type FailureThresholds struct {
	// RepeatCeiling: same task or same failure-class recurring this many
	// cycles with no landed progress ⇒ non-progress (system-level halt).
	RepeatCeiling int `json:"repeat_ceiling,omitempty"`
	// VerifiedNotLandedCeiling: verified-green (audit PASS + ACS PASS) cycles
	// that do not land this many times ⇒ non-progress (the clean-exit signature).
	VerifiedNotLandedCeiling int `json:"verified_not_landed_ceiling,omitempty"`
	// TaskRetryCeiling: a task-level failure count reaching this ⇒ quarantine
	// (stop re-picking the poison todo).
	TaskRetryCeiling int `json:"task_retry_ceiling,omitempty"`
}

// SystemFailurePolicy is the resolved decision policy.
type SystemFailurePolicy struct {
	Categories         map[string]FailureCategory `json:"categories,omitempty"`
	Thresholds         FailureThresholds          `json:"thresholds,omitempty"`
	OnTaskRetryCeiling string                     `json:"on_task_retry_ceiling,omitempty"`
	OnSystemLevel      string                     `json:"on_system_level,omitempty"`
}

// DefaultSystemFailurePolicy is the compiled default surfaced when the
// failure_policy block is absent. It matches ADR-0072's table so behavior is
// correct without editing the checked-in policy.json (mirrors the
// gates/observer default pattern).
func DefaultSystemFailurePolicy() SystemFailurePolicy {
	return SystemFailurePolicy{
		Categories: map[string]FailureCategory{
			CategoryVerdictIncoherence: {Level: LevelSystem, Action: ActionHaltAndDiagnose, FixType: "pipeline-repair", Floor: true,
				Signature: "recorded FAIL/WARN but on-disk audit AND acs verdicts are PASS"},
			CategoryInfraSystemic: {Level: LevelSystem, Action: ActionHaltAndDiagnose, FixType: "pipeline-repair", Floor: true,
				Signature: "all CLI families exhausted / systemic infrastructure teardown"},
			CategoryTransportHang: {Level: LevelSystem, Action: ActionHaltAndDiagnose, FixType: "pipeline-repair",
				Signature: "exit-transport hang: session ended without a well-formed deliverable or verdict"},
			CategoryNonProgress: {Level: LevelSystem, Action: ActionHaltAndDiagnose, FixType: "pipeline-repair",
				Signature: "same task/failure-class recurs >= repeat_ceiling with no landed progress, OR verified-green not landed >= verified_not_landed_ceiling"},
			CategoryCodeBuildFail:   {Level: LevelTask, Action: ActionRetryWithFix, FixType: "build-repair", MaxRetries: 2},
			CategoryCodeAuditFail:   {Level: LevelTask, Action: ActionRetryWithFix, FixType: "address-audit-findings", MaxRetries: 2},
			CategoryIntentMalformed: {Level: LevelTask, Action: ActionDeferOrQuarantine, FixType: "reintent"},
		},
		Thresholds:         FailureThresholds{RepeatCeiling: 2, VerifiedNotLandedCeiling: 2, TaskRetryCeiling: 2},
		OnTaskRetryCeiling: "quarantine",
		OnSystemLevel:      "halt-loop-and-escalate",
	}
}

// floorCategories are the keys whose {level:system, action:halt-and-diagnose,
// floor:true} shape is non-negotiable. FailurePolicyConfig re-stamps them even
// if operator policy or a typo tries to demote them — mirroring how ShipFloor
// always re-appends "audit".
var floorCategories = []string{CategoryVerdictIncoherence, CategoryInfraSystemic}

// FailurePolicyConfig returns the failure_policy with compiled defaults
// resolved and the floor invariant enforced. An absent block yields
// DefaultSystemFailurePolicy(); a partial block merges over the defaults
// per-category and per-threshold. Malformed levels/actions are rejected explicitly.
func (p Policy) FailurePolicyConfig() (SystemFailurePolicy, error) {
	out := DefaultSystemFailurePolicy()
	if c := p.SystemFailurePolicy; c != nil {
		// Per-category merge: an override replaces the whole category row, but
		// categories not mentioned survive from defaults.
		for name, cat := range c.Categories {
			if err := validateCategory(name, cat); err != nil {
				return SystemFailurePolicy{}, err
			}
			out.Categories[name] = cat
		}
		// Per-threshold merge: only positive overrides win; zero ⇒ keep default.
		if c.Thresholds.RepeatCeiling > 0 {
			out.Thresholds.RepeatCeiling = c.Thresholds.RepeatCeiling
		}
		if c.Thresholds.VerifiedNotLandedCeiling > 0 {
			out.Thresholds.VerifiedNotLandedCeiling = c.Thresholds.VerifiedNotLandedCeiling
		}
		if c.Thresholds.TaskRetryCeiling > 0 {
			out.Thresholds.TaskRetryCeiling = c.Thresholds.TaskRetryCeiling
		}
		if c.OnTaskRetryCeiling != "" {
			out.OnTaskRetryCeiling = c.OnTaskRetryCeiling
		}
		if c.OnSystemLevel != "" {
			out.OnSystemLevel = c.OnSystemLevel
		}
	}
	// Enforce the floor invariant: the non-negotiable halt categories are
	// re-stamped to their canonical shape no matter what the override said.
	def := DefaultSystemFailurePolicy()
	for _, key := range floorCategories {
		out.Categories[key] = def.Categories[key]
	}
	return out, nil
}

// IsFloor reports whether the named category is a Go-enforced floor halt.
func (fp SystemFailurePolicy) IsFloor(category string) bool {
	c, ok := fp.Categories[category]
	return ok && c.Floor
}

// IsSystemLevel reports whether the named category halts the loop.
func (fp SystemFailurePolicy) IsSystemLevel(category string) bool {
	c, ok := fp.Categories[category]
	return ok && c.Level == LevelSystem
}

func validateCategory(name string, c FailureCategory) error {
	switch c.Level {
	case LevelSystem, LevelTask:
	default:
		return fmt.Errorf("policy: failure_policy category %q has unknown level %q (want system|task)", name, c.Level)
	}
	switch c.Action {
	case ActionHaltAndDiagnose, ActionRetryWithFix, ActionDeferOrQuarantine:
	default:
		return fmt.Errorf("policy: failure_policy category %q has unknown action %q", name, c.Action)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("policy: failure_policy category %q max_retries must be >= 0, got %d", name, c.MaxRetries)
	}
	return nil
}
