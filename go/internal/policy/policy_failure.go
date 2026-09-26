package policy

import "fmt"

// Level and action vocabularies of the failure_policy table.
// See ADR-0072.
const (
	LevelSystem = "system"
	LevelTask   = "task"

	ActionHaltAndDiagnose   = "halt-and-diagnose"
	ActionRetryWithFix      = "retry-with-fix"
	ActionDeferOrQuarantine = "defer-or-quarantine"
)

// Category keys of the failure_policy table; the dossier layer maps failureadapter classifications onto them.
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
	Level      string `json:"level"`
	Action     string `json:"action"`
	FixType    string `json:"fix_type,omitempty"`
	Signature  string `json:"signature,omitempty"`
	Floor      bool   `json:"floor,omitempty"`       // Go always halts; not overridable
	MaxRetries int    `json:"max_retries,omitempty"` // task-level retries before quarantine
}

// FailureThresholds are the non-progress, retry and halt counters.
type FailureThresholds struct {
	// RepeatCeiling: the same task or failure class recurring this many cycles without landed progress is non-progress.
	RepeatCeiling int `json:"repeat_ceiling,omitempty"`
	// VerifiedNotLandedCeiling: this many verified-green cycles that do not land is non-progress.
	VerifiedNotLandedCeiling int `json:"verified_not_landed_ceiling,omitempty"`
	// TaskRetryCeiling: a task-level failure count reaching this quarantines the todo.
	TaskRetryCeiling int `json:"task_retry_ceiling,omitempty"`
	// GuardClassHaltCeiling: this many guard-abort failures in one batch halt it,
	// since guard aborts are pipeline failures, never task defects.
	GuardClassHaltCeiling int `json:"guard_class_halt_ceiling,omitempty"`
	// IdenticalFingerprintHaltCeiling: one failure fingerprint recurring this many times in a batch halts it.
	IdenticalFingerprintHaltCeiling int `json:"identical_fingerprint_halt_ceiling,omitempty"`
	// UnexplainedFailuresHaltCeiling: this many failures with no machine-readable reason in a batch halt it.
	UnexplainedFailuresHaltCeiling int `json:"unexplained_failures_halt_ceiling,omitempty"`
	// ConsecutiveFailuresHaltCeiling: this many back-to-back failing cycles, any fingerprints, halt the batch.
	ConsecutiveFailuresHaltCeiling int `json:"consecutive_failures_halt_ceiling,omitempty"`
	// BuildDeepEscalateAtFailures: an item with this many failures builds next at the
	// deep tier (raise-only; the envelope still clamps).
	BuildDeepEscalateAtFailures int `json:"build_deep_escalate_at_failures,omitempty"`
}

// SystemFailurePolicy is the failure_policy decision table (Policy.FailurePolicy resolves failure_floor instead).
type SystemFailurePolicy struct {
	Categories         map[string]FailureCategory `json:"categories,omitempty"`
	Thresholds         FailureThresholds          `json:"thresholds,omitempty"`
	OnTaskRetryCeiling string                     `json:"on_task_retry_ceiling,omitempty"`
	OnSystemLevel      string                     `json:"on_system_level,omitempty"`
}

// DefaultSystemFailurePolicy returns the compiled decision table used when the block is absent.
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
		Thresholds:         FailureThresholds{RepeatCeiling: 2, VerifiedNotLandedCeiling: 2, TaskRetryCeiling: 2, GuardClassHaltCeiling: 2, IdenticalFingerprintHaltCeiling: 3, UnexplainedFailuresHaltCeiling: 3, ConsecutiveFailuresHaltCeiling: 3, BuildDeepEscalateAtFailures: 1},
		OnTaskRetryCeiling: "quarantine",
		OnSystemLevel:      "halt-loop-and-escalate",
	}
}

// floorCategories always halt: FailurePolicyConfig re-stamps their default rows over any override.
var floorCategories = []string{CategoryVerdictIncoherence, CategoryInfraSystemic}

// FailurePolicyConfig merges failure_policy over the defaults, rejecting unknown levels and actions.
func (p Policy) FailurePolicyConfig() (SystemFailurePolicy, error) {
	out := DefaultSystemFailurePolicy()
	if c := p.SystemFailurePolicy; c != nil {
		// An override replaces a whole category row; unmentioned rows keep their defaults.
		for name, cat := range c.Categories {
			if err := validateCategory(name, cat); err != nil {
				return SystemFailurePolicy{}, err
			}
			out.Categories[name] = cat
		}
		if c.Thresholds.RepeatCeiling > 0 {
			out.Thresholds.RepeatCeiling = c.Thresholds.RepeatCeiling
		}
		if c.Thresholds.VerifiedNotLandedCeiling > 0 {
			out.Thresholds.VerifiedNotLandedCeiling = c.Thresholds.VerifiedNotLandedCeiling
		}
		if c.Thresholds.TaskRetryCeiling > 0 {
			out.Thresholds.TaskRetryCeiling = c.Thresholds.TaskRetryCeiling
		}
		if c.Thresholds.GuardClassHaltCeiling > 0 {
			out.Thresholds.GuardClassHaltCeiling = c.Thresholds.GuardClassHaltCeiling
		}
		if c.Thresholds.IdenticalFingerprintHaltCeiling > 0 {
			out.Thresholds.IdenticalFingerprintHaltCeiling = c.Thresholds.IdenticalFingerprintHaltCeiling
		}
		if c.Thresholds.UnexplainedFailuresHaltCeiling > 0 {
			out.Thresholds.UnexplainedFailuresHaltCeiling = c.Thresholds.UnexplainedFailuresHaltCeiling
		}
		if c.Thresholds.ConsecutiveFailuresHaltCeiling > 0 {
			out.Thresholds.ConsecutiveFailuresHaltCeiling = c.Thresholds.ConsecutiveFailuresHaltCeiling
		}
		if c.Thresholds.BuildDeepEscalateAtFailures > 0 {
			out.Thresholds.BuildDeepEscalateAtFailures = c.Thresholds.BuildDeepEscalateAtFailures
		}
		if c.OnTaskRetryCeiling != "" {
			out.OnTaskRetryCeiling = c.OnTaskRetryCeiling
		}
		if c.OnSystemLevel != "" {
			out.OnSystemLevel = c.OnSystemLevel
		}
	}
	// The floor categories are re-stamped whatever the override said.
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

// RetryPolicyFor returns the category's row; callers treat ok=false as "no retry", never a default allow.
func (fp SystemFailurePolicy) RetryPolicyFor(category string) (FailureCategory, bool) {
	c, ok := fp.Categories[category]
	return c, ok
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
