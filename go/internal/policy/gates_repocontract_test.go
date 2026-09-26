package policy

import "testing"

func TestGatesConfig_RepoContractGateDefaultEnforce(t *testing.T) {
	if got := (Policy{}).GatesConfig().RepoContractGate; got != "enforce" {
		t.Fatalf("default = %q, want enforce", got)
	}
	p := Policy{Gates: &GatesPolicy{RepoContractGate: "off"}}
	if got := p.GatesConfig().RepoContractGate; got != "off" {
		t.Fatalf("operator off = %q, want off", got)
	}
}
