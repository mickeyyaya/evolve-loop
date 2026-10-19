package policy

import "fmt"

const (
	LevelSystem = "system"
	LevelTask   = "task"

	ActionHaltAndDiagnose   = "halt-and-diagnose"
	ActionRetryWithFix      = "retry-with-fix"
	ActionDeferOrQuarantine = "defer-or-quarantine"
)

const (
	CategoryVerdictIncoherence = "verdict-incoherence"
	CategoryInfraSystemic      = "infra-systemic"
	CategoryTransportHang      = "transport-hang"
	CategoryNonProgress        = "non-progress"
	CategoryCodeBuildFail      = "code-build-fail"
	CategoryCodeAuditFail      = "code-audit-fail"
	CategoryIntentMalformed    = "intent-malformed"
)

type FailureCategory struct {
	Level      string `json:"level"`
	Action     string `json:"action"`
	FixType    string `json:"fix_type,omitempty"`
	Signature  string `json:"signature,omitempty"`
	Floor      bool   `json:"floor,omitempty"`
	MaxRetries int    `json:"max_retries,omitempty"`
}

type FailureThresholds struct {
	RepeatCeiling                   int `json:"repeat_ceiling,omitempty"`
	VerifiedNotLandedCeiling        int `json:"verified_not_landed_ceiling,omitempty"`
	TaskRetryCeiling                int `json:"task_retry_ceiling,omitempty"`
	GuardClassHaltCeiling           int `json:"guard_class_halt_ceiling,omitempty"`
	IdenticalFingerprintHaltCeiling int `json:"identical_fingerprint_halt_ceiling,omitempty"`
	UnexplainedFailuresHaltCeiling  int `json:"unexplained_failures_halt_ceiling,omitempty"`
	ConsecutiveFailuresHaltCeiling  int `json:"consecutive_failures_halt_ceiling,omitempty"`
	LaneDeferralHaltCeiling         int `json:"lane_deferral_halt_ceiling,omitempty"`
	ZeroShipHalt                    int `json:"zero_ship_halt,omitempty"`
	BuildDeepEscalateAtFailures     int `json:"build_deep_escalate_at_failures,omitempty"`
}

type SystemFailurePolicy struct {
	Categories         map[string]FailureCategory `json:"categories,omitempty"`
	Thresholds         FailureThresholds          `json:"thresholds,omitempty"`
	OnTaskRetryCeiling string                     `json:"on_task_retry_ceiling,omitempty"`
	OnSystemLevel      string                     `json:"on_system_level,omitempty"`
}

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
		Thresholds:         FailureThresholds{RepeatCeiling: 2, VerifiedNotLandedCeiling: 2, TaskRetryCeiling: 2, GuardClassHaltCeiling: 2, IdenticalFingerprintHaltCeiling: 3, UnexplainedFailuresHaltCeiling: 3, ConsecutiveFailuresHaltCeiling: 3, LaneDeferralHaltCeiling: 3, ZeroShipHalt: 2, BuildDeepEscalateAtFailures: 1},
		OnTaskRetryCeiling: "quarantine",
		OnSystemLevel:      "halt-loop-and-escalate",
	}
}

var floorCategories = []string{CategoryVerdictIncoherence, CategoryInfraSystemic}

func (p Policy) FailurePolicyConfig() (SystemFailurePolicy, error) {
	out := DefaultSystemFailurePolicy()
	if c := p.SystemFailurePolicy; c != nil {
		for name, cat := range c.Categories {
			if err := validateCategory(name, cat); err != nil {
				return SystemFailurePolicy{}, err
			}
			out.Categories[name] = cat
		}
		out.Thresholds = mergeThresholds(out.Thresholds, c.Thresholds)
		if c.OnTaskRetryCeiling != "" {
			out.OnTaskRetryCeiling = c.OnTaskRetryCeiling
		}
		if c.OnSystemLevel != "" {
			out.OnSystemLevel = c.OnSystemLevel
		}
	}
	def := DefaultSystemFailurePolicy()
	for _, key := range floorCategories {
		out.Categories[key] = def.Categories[key]
	}
	return out, nil
}

func mergeThresholds(base, override FailureThresholds) FailureThresholds {
	out := base
	out.RepeatCeiling = positiveOr(override.RepeatCeiling, base.RepeatCeiling)
	out.VerifiedNotLandedCeiling = positiveOr(override.VerifiedNotLandedCeiling, base.VerifiedNotLandedCeiling)
	out.TaskRetryCeiling = positiveOr(override.TaskRetryCeiling, base.TaskRetryCeiling)
	out.GuardClassHaltCeiling = positiveOr(override.GuardClassHaltCeiling, base.GuardClassHaltCeiling)
	out.IdenticalFingerprintHaltCeiling = positiveOr(override.IdenticalFingerprintHaltCeiling, base.IdenticalFingerprintHaltCeiling)
	out.UnexplainedFailuresHaltCeiling = positiveOr(override.UnexplainedFailuresHaltCeiling, base.UnexplainedFailuresHaltCeiling)
	out.ConsecutiveFailuresHaltCeiling = positiveOr(override.ConsecutiveFailuresHaltCeiling, base.ConsecutiveFailuresHaltCeiling)
	out.LaneDeferralHaltCeiling = positiveOr(override.LaneDeferralHaltCeiling, base.LaneDeferralHaltCeiling)
	out.ZeroShipHalt = positiveOr(override.ZeroShipHalt, base.ZeroShipHalt)
	out.BuildDeepEscalateAtFailures = positiveOr(override.BuildDeepEscalateAtFailures, base.BuildDeepEscalateAtFailures)
	return out
}

func positiveOr(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func (fp SystemFailurePolicy) IsFloor(category string) bool {
	c, ok := fp.Categories[category]
	return ok && c.Floor
}

func (fp SystemFailurePolicy) RetryPolicyFor(category string) (FailureCategory, bool) {
	c, ok := fp.Categories[category]
	return c, ok
}

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
