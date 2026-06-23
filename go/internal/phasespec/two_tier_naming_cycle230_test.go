package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// Cycle-230 companion tests to TestBugRepro_Cycle229_TwoTierNamingMissing
// (task phase-naming-lint). The anchor test covers the rejection criterion;
// these cover the acceptance + edge axes (adversarial-testing SKILL §6) so a
// fix that over-restricts (rejecting valid multi-word names) is also caught.
//
// DO NOT MODIFY (builder contract): make these pass by changing
// go/internal/phasespec/validate.go only.

// TestTwoTierNaming_MultiWordAccepted: multi-word kebab-case user phase names
// must NOT receive a "multi-word" naming violation. Pre-existing GREEN at RED
// baseline (guards against an over-broad fix regex).
func TestTwoTierNaming_MultiWordAccepted(t *testing.T) {
	multiWordNames := []string{"bug-reproduction", "my-check", "security-scan", "a-b"}
	for _, name := range multiWordNames {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		for _, v := range violations {
			if strings.Contains(v, "multi-word") {
				t.Errorf("ValidateUserSpec(%q): unexpected multi-word violation %q — valid kebab-case multi-word names must be accepted", name, v)
			}
		}
	}
}

// TestTwoTierNaming_MalformedRejected: names that superficially look hyphenated
// but are not valid <object>-<action> kebab-case must produce at least one
// violation. "scanner-" passes the legacy nameRE (trailing hyphen allowed by
// ^[a-z][a-z0-9-]*$) but must fail the two-tier gate ^[a-z]+(-[a-z]+)+$.
func TestTwoTierNaming_MalformedRejected(t *testing.T) {
	malformed := []string{"scanner-", "scan--go"}
	for _, name := range malformed {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		if len(violations) == 0 {
			t.Errorf("ValidateUserSpec(%q): expected at least one naming violation for malformed kebab-case, got none", name)
		}
	}
}
