package policy

type IsolationPolicy struct {
	I2 IsolationInvariantPolicy `json:"i2,omitempty"`
}

type IsolationInvariantPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) IsolationEnvStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I2.Stage
}
