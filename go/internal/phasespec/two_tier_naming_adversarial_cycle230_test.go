package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestTwoTierNaming_DigitsRejected_Amp(t *testing.T) {
	digitNames := []string{
		"phase2-check",
		"my-phase2",
		"a-1-b",
		"2-check",
	}
	for _, name := range digitNames {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		if len(violations) == 0 {
			t.Errorf("ValidateUserSpec(%q): expected at least one violation for digit-containing name, got none", name)
		}
	}
}

func TestTwoTierNaming_ThreePlusWordsAccepted_Amp(t *testing.T) {
	threeWordNames := []string{
		"a-b-c",
		"security-scan-deep",
		"bug-reproduction-scan",
		"a-b-c-d-e",
	}
	for _, name := range threeWordNames {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		for _, v := range violations {
			if strings.Contains(v, "multi-word") {
				t.Errorf("ValidateUserSpec(%q): unexpected multi-word violation %q — three-or-more-word names must be accepted", name, v)
			}
		}
	}
}

func TestTwoTierNaming_UnderscoreAndSpecialRejected_Amp(t *testing.T) {
	badSep := []string{
		"my_phase",
		"my.phase",
		"my phase",
		"my-phase_b",
	}
	for _, name := range badSep {
		spec := phasespec.PhaseSpec{
			Name:     name,
			Optional: true,
			Kind:     "llm",
		}
		violations := phasespec.ValidateUserSpec(spec)
		if len(violations) == 0 {
			t.Errorf("ValidateUserSpec(%q): expected at least one violation for invalid-separator name, got none", name)
		}
	}
}
