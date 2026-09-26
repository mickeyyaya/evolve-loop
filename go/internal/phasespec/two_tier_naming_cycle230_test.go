package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

// "scanner-" passes nameRE (a trailing hyphen is allowed), so only twoTierNameRE catches it.
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
