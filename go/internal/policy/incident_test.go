package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestIncidentEvidenceBudgetBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		json string
		want int
	}{
		{"absent", `{}`, 256 * 1024},
		{"empty-block", `{"incident":{}}`, 256 * 1024},
		{"set", `{"incident":{"evidence":{"budget_bytes":5600}}}`, 5600},
		{"non-positive", `{"incident":{"evidence":{"budget_bytes":-1}}}`, 256 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := policy.Parse([]byte(tc.json))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := p.IncidentEvidenceBudgetBytes(); got != tc.want {
				t.Errorf("IncidentEvidenceBudgetBytes() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIncidentPolicy_TypesCarryTheBudget(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Incident: &policy.IncidentPolicy{Evidence: &policy.IncidentEvidencePolicy{BudgetBytes: 7}}}
	if got := p.IncidentEvidenceBudgetBytes(); got != 7 {
		t.Errorf("IncidentEvidenceBudgetBytes() = %d, want 7", got)
	}
}
