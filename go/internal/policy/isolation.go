package policy

type IsolationPolicy struct {
	I2 IsolationInvariantPolicy `json:"i2,omitempty"`
	I3 IsolationInvariantPolicy `json:"i3,omitempty"`
	I5 IsolationInvariantPolicy `json:"i5,omitempty"`
	I8 IsolationInvariantPolicy `json:"i8,omitempty"`

	LineageMax int `json:"lineage_max,omitempty"`
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

func (p Policy) IsolationFlowStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I8.Stage
}

func (p Policy) IsolationReadStage() string {
	if p.Isolation == nil {
		return ""
	}
	return p.Isolation.I5.Stage
}

func (p Policy) IsolationLineageMax() int {
	if p.Isolation == nil {
		return 0
	}
	return p.Isolation.LineageMax
}
