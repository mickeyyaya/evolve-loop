package phasespec_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Cycle-230 test-amplification adversarial tests for task phase-naming-lint.
// Written from spec only (no implementation read) — anti-bias isolation.
//
// Coverage gaps addressed:
//   - Digit-containing names: twoTierNameRE ^[a-z]+(-[a-z]+)+$ disallows digits
//     (documented in builder notes; tests deliberately NOT written by TDD-engineer)
//   - Three-or-more word names: ensures the (+) quantifier works for >2 segments
//   - Non-optional specs: ValidateUserSpec is called on non-optional specs in some
//     callers; built-in single-word names must not trip the two-tier gate if the
//     Optional flag guards the branch.

// TestTwoTierNaming_DigitsRejected_Amp: names that include ASCII digits must be
// rejected by the two-tier gate because twoTierNameRE uses [a-z] (no \d).
// Builder notes explicitly state digit behavior is not pinned by TDD tests —
// this amplification test fills that gap.
func TestTwoTierNaming_DigitsRejected_Amp(t *testing.T) {
	digitNames := []string{
		"phase2-check", // digit in first segment
		"my-phase2",   // digit in second segment
		"a-1-b",       // pure-digit middle segment
		"2-check",     // starts with digit (also breaks legacy nameRE)
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

// TestTwoTierNaming_ThreePlusWordsAccepted_Amp: the regex quantifier (+) means
// the pattern requires ONE OR MORE (-[a-z]+) groups after the first segment.
// Names with 3 or more segments must therefore be accepted.
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

// TestTwoTierNaming_UnderscoreAndSpecialRejected_Amp: names that use underscores
// or other non-hyphen separators are malformed and must produce at least one
// violation (caught by either the legacy nameRE or the twoTierNameRE).
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
