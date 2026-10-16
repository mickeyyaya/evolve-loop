package policy

const defaultContextFillWarnPct = 60

type ContextFillPolicy struct {
	WarnThresholdPct int `json:"warn_threshold_pct"`
}

type ContextFillConfig struct {
	WarnThresholdPct int
}

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
