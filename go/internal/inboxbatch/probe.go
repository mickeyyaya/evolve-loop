package inboxbatch

type ProbeKind string

const (
	ProbeTestExists   ProbeKind = "test_exists"
	ProbeSymbolExists ProbeKind = "symbol_exists"
	ProbeTextAbsent   ProbeKind = "text_absent"
	ProbePolicyValue  ProbeKind = "policy_value"
)

func ProbeKinds() []ProbeKind {
	return []ProbeKind{ProbeTestExists, ProbeSymbolExists, ProbeTextAbsent, ProbePolicyValue}
}

type Probe struct {
	Kind   ProbeKind `json:"kind"`
	Target string    `json:"target"`
	Value  string    `json:"value,omitempty"`
}
