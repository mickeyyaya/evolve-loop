package phasecontract

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// A derivation is declared in the registry alone; it reaches the built-in contract by overlay and a user
// phase's contract by projection.
func TestDerivedFrom_ReachesTheContractFromTheRegistry(t *testing.T) {
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{
		{Name: "triage", Role: "triage", Outputs: phasespec.IO{
			Files:       []string{".evolve/runs/cycle-{cycle}/triage-report.md", ".evolve/runs/cycle-{cycle}/triage-decision.json"},
			AgentOwed:   []string{"triage-decision.json"},
			DerivedFrom: map[string]string{"triage-decision.json": "triage-report.md"},
		}},
		{Name: "user-selector", Outputs: phasespec.IO{
			Files:       []string{".evolve/runs/cycle-{cycle}/user-selector-report.md", ".evolve/runs/cycle-{cycle}/pick.json"},
			AgentOwed:   []string{"pick.json"},
			DerivedFrom: map[string]string{"pick.json": "user-selector-report.md"},
		}},
	})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	r := NewCatalogResolver(cat.Get)
	for phase, want := range map[string]string{"triage": "triage-report.md", "user-selector": "user-selector-report.md"} {
		c, ok := r.Resolve(phase)
		if !ok {
			t.Fatalf("%s: no contract", phase)
		}
		if len(c.DerivedFrom) != 1 {
			t.Fatalf("%s: DerivedFrom = %v", phase, c.DerivedFrom)
		}
		for owed, src := range c.DerivedFrom {
			if src != want {
				t.Errorf("%s: %s derived from %s, want %s", phase, owed, src, want)
			}
		}
	}
}
