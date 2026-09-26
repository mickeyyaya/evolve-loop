package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestTwoTierNaming_DigitsRejected_Amp(t *testing.T) {
	digitNames := []string{
		"phase2-check", // digit in first segment
		"my-phase2",    // digit in second segment
		"a-1-b",        // pure-digit middle segment
		"2-check",      // starts with digit (also breaks legacy nameRE)
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
		"my_phase",   // underscore separator
		"my.phase",   // period separator
		"my phase",   // space separator
		"my-phase_b", // mixed valid and invalid separators
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
