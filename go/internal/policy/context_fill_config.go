package policy

// context_fill_config.go — the .evolve/policy.json "context_fill" block
// (cycle-1444, task `context-fill-warn-threshold`). Shape mirrors
// ParallelEvaluatePolicy, the canonical sub-block resolution pattern in this
// package: a pointer block on Policy plus a pure resolver that applies
// built-in defaults. Config-as-code, no flags — see
// [[phase_settings_from_config_not_code]].

// defaultContextFillWarnPct is the built-in warn threshold: a launch past 60%
// of its effective context window is close enough to compaction to be worth an
// operator's attention.
const defaultContextFillWarnPct = 60

// ContextFillPolicy is the operator-facing "context_fill" block.
type ContextFillPolicy struct {
	// WarnThresholdPct is the percentage of the effective context window above
	// which a launch WARNs. Absent/zero/out-of-range (≤0 or >100) ⇒ 60. The
	// range check is load-bearing rather than cosmetic: 0 would arm the WARN on
	// every launch and 900 would disarm it permanently, and both are typo shapes
	// an operator would never intend.
	WarnThresholdPct int `json:"warn_threshold_pct"`
}

// ContextFillConfig is the resolved context-fill configuration with defaults
// applied.
type ContextFillConfig struct {
	WarnThresholdPct int
}

// ContextFillConfig returns context-fill configuration with built-in defaults
// resolved. The zero-value Policy{} yields the safe default (60). Operator
// input is never accepted verbatim: only a threshold inside 1–100 overrides,
// anything else falls back to the visible built-in. Pure.
func (p Policy) ContextFillConfig() ContextFillConfig {
	c := ContextFillConfig{WarnThresholdPct: defaultContextFillWarnPct}
	if p.ContextFill == nil {
		return c
	}
	if v := p.ContextFill.WarnThresholdPct; v > 0 && v <= 100 {
		c.WarnThresholdPct = v
	}
	return c
}
