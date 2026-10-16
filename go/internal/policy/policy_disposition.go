package policy

type FailureDispositionPolicy struct {
	Stage     string  `json:"stage,omitempty"`
	Threshold int     `json:"threshold,omitempty"`
	Step      float64 `json:"step,omitempty"`
	Cap       float64 `json:"cap,omitempty"`
}

type FailureDispositionConfig struct {
	Stage     string
	Threshold int
	Step      float64
	Cap       float64
}

func (c FailureDispositionConfig) Enforce() bool { return c.Stage == "enforce" }

func (p Policy) FailureDispositionConfig() FailureDispositionConfig {
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
	if p.FailureDisposition.Threshold > 0 {
		c.Threshold = p.FailureDisposition.Threshold
	}
	if p.FailureDisposition.Step > 0 {
		c.Step = p.FailureDisposition.Step
	}
	if p.FailureDisposition.Cap > 0 {
		c.Cap = p.FailureDisposition.Cap
	}
	return c
}
