package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestBugRepro_Cycle229_TwoTierNamingMissing reproduces the bug described in
// scout-report.md finding #2: ValidateUserSpec accepts single-word user phase
// names (e.g. "scanner") because it only checks nameRE (^[a-z][a-z0-9-]*$)
// and lacks the two-tier enforcement gate (^[a-z]+(-[a-z]+)+$).
//
// FAIL on main tree (pre-fix): no violation returned for "scanner".
// PASS on worktree cycle-229 (post-fix): violation contains "multi-word".
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
