package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestBugRepro_Cycle229_TwoTierNamingMissing(t *testing.T) {
	singleWordNames := []string{"scanner", "analyzer", "a", "review"}
	for _, name := range singleWordNames {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		hasMultiWordViolation := false
		for _, v := range violations {
			if strings.Contains(v, "multi-word") {
				hasMultiWordViolation = true
				break
			}
		}
		if !hasMultiWordViolation {
			t.Errorf("ValidateUserSpec(%q): expected a violation containing %q, got violations=%v", name, "multi-word", violations)
		}
	}
}
