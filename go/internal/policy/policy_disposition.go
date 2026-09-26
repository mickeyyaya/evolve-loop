package policy

// FailureDispositionPolicy is the "failure_disposition" block for the escalation boundary applier.
// See ADR-0076.
type FailureDispositionPolicy struct {
	// Stage is "shadow" (report only) or "enforce" (mutate the inbox); absent follows chronicle.escalation.
	Stage string `json:"stage,omitempty"`
	// Threshold is the recurrence count at or above which a pattern escalates.
	Threshold int `json:"threshold,omitempty"`
	// Step is the weight added per extra occurrence.
	Step float64 `json:"step,omitempty"`
	// Cap is the maximum weight escalation may reach.
	Cap float64 `json:"cap,omitempty"`
}

// FailureDispositionConfig is the resolved configuration with defaults applied.
type FailureDispositionConfig struct {
	Stage     string
	Threshold int
	Step      float64
	Cap       float64
}

// Enforce reports whether the applier may mutate the inbox; every stage but "enforce" is report-only.
func (c FailureDispositionConfig) Enforce() bool { return c.Stage == "enforce" }

// FailureDispositionConfig resolves the block, defaulting the stage to chronicle.escalation.
func (p Policy) FailureDispositionConfig() FailureDispositionConfig {
	// These defaults must match recurrence.DefaultEscalationPolicy.
	c := FailureDispositionConfig{
		Stage:     p.ChronicleConfig().Escalation,
		Threshold: 2,
		Step:      0.03,
		Cap:       0.99,
	}
	if p.FailureDisposition == nil {
		return c
	}
	if p.FailureDisposition.Stage != "" {
		c.Stage = p.FailureDisposition.Stage
	}
	if p.FailureDisposition.Threshold != 0 {
		c.Threshold = p.FailureDisposition.Threshold
	}
	if p.FailureDisposition.Step != 0 {
		c.Step = p.FailureDisposition.Step
	}
	if p.FailureDisposition.Cap != 0 {
		c.Cap = p.FailureDisposition.Cap
	}
	return c
}
