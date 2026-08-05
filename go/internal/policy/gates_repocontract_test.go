package policy

import "testing"

// TestGatesConfig_RepoContractGateDefaultEnforce pins the enforce-default
// deviation from shadow-first (see the field doc: the pack is existing
// deterministic FP≈0 repo tests whose breakage IS a red main).
func TestGatesConfig_RepoContractGateDefaultEnforce(t *testing.T) {
	if got := (Policy{}).GatesConfig().RepoContractGate; got != "enforce" {
		t.Fatalf("default = %q, want enforce", got)
	}
	p := Policy{Gates: &GatesPolicy{RepoContractGate: "off"}}
	if got := p.GatesConfig().RepoContractGate; got != "off" {
		t.Fatalf("operator off = %q, want off", got)
	}
}
