package policy

const defaultContextFillWarnPct = 60

// ContextFillPolicy is the "context_fill" block. See docs/architecture/policy-config.md.
type ContextFillPolicy struct {
	// WarnThresholdPct is the share of the effective context window above which a
	// launch WARNs; outside 1..100 means 60, since 0 or 900 would be typos that break the WARN.
	WarnThresholdPct int `json:"warn_threshold_pct"`
}

// ContextFillConfig is the resolved context-fill configuration with defaults applied.
type ContextFillConfig struct {
	WarnThresholdPct int
}

// ContextFillConfig returns the context_fill block; only a threshold in 1..100 overrides the default 60.
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
