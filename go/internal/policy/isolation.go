package policy

type IsolationPolicy struct {
	I2 IsolationInvariantPolicy `json:"i2,omitempty"`
	I3 IsolationInvariantPolicy `json:"i3,omitempty"`
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

func (p Policy) IsolationExecStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I3.Stage
}
