package ciparity

// graduation_prescription_test.go — cycle-1329 RED contract for task
// audit-warn-prescription-gate (scout-report.md hypothesis 1): relocate the
// build-entry seam's `graduationPrescription` helper
// (go/internal/core/phase_bindings_graduation.go:81, currently unexported to
// package core) into this shared `ciparity` package as an EXPORTED
// `GraduationPrescription`, so BOTH the build seam (core) and the audit seam
// (phases/audit) can render the identical copy-pasteable fix for an
// ungraduated package — closing the gap where audit's offender line only
// said "add it + an apicover_named_test.go" while build's abort_reason
// spelled out the exact two edits.
//
// Contract under test (Builder implements; this file must not be modified):
//
//	func GraduationPrescription(pkgs []string) string
//
// Per-package output is a copy-pasteable two-step fix:
//  1. "append this line to go/.apicover-enforce:  <pkg>"
//  2. "create go/<dir>/apicover_named_test.go naming every exported symbol
//     of the package in a real assertion that executes it..."
//
// EXCEPT a recursive "..."-suffixed pattern names no single directory, so
// step 2 instead reads "add an apicover_named_test.go in EACH package the
// pattern covers..." (AC3 — the branch preserved verbatim from
// phase_bindings_graduation.go:86-92, the exact behavior this relocation
// must not regress).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestGraduationPrescription_EmitsAppendLineAndTestPath
//   - Negative : TestGraduationPrescription_EmptyInputReturnsEmptyString
//   - Edge     : TestGraduationPrescription_PatternSuffixSkipsBogusPath (AC3:
//     the "..." recursive-pattern branch must not degrade to a bogus
//     ".../apicover_named_test.go" path)
//   - Semantic : TestGraduationPrescription_MultiplePackagesEachGetOwnBlock
//     (distinct behavior from the single-package case, not a restatement)

import (
	"strings"
	"testing"
)

// TestGraduationPrescription_EmitsAppendLineAndTestPath is the positive case:
// a single ordinary package must render both the literal .apicover-enforce
// append line and the literal apicover_named_test.go path to create.
func TestGraduationPrescription_EmitsAppendLineAndTestPath(t *testing.T) {
	got := GraduationPrescription([]string{"./internal/brandnew"})
	if !strings.Contains(got, "append this line to go/.apicover-enforce:  ./internal/brandnew") {
		t.Errorf("GraduationPrescription([./internal/brandnew]) = %q, want it to contain the literal .apicover-enforce append line", got)
	}
	if !strings.Contains(got, "go/internal/brandnew/apicover_named_test.go") {
		t.Errorf("GraduationPrescription([./internal/brandnew]) = %q, want it to contain the literal apicover_named_test.go path", got)
	}
	if !strings.Contains(got, "naming every exported symbol") {
		t.Errorf("GraduationPrescription([./internal/brandnew]) = %q, want the naming-every-exported-symbol instruction", got)
	}
}

// TestGraduationPrescription_EmptyInputReturnsEmptyString is the negative
// case: no ungraduated packages means no prescription — the strongest
// anti-no-op guard against a naive implementation that always emits a fixed
// template regardless of input.
func TestGraduationPrescription_EmptyInputReturnsEmptyString(t *testing.T) {
	if got := GraduationPrescription(nil); got != "" {
		t.Errorf("GraduationPrescription(nil) = %q, want \"\" (nothing to prescribe)", got)
	}
	if got := GraduationPrescription([]string{}); got != "" {
		t.Errorf("GraduationPrescription([]) = %q, want \"\" (nothing to prescribe)", got)
	}
}

// TestGraduationPrescription_PatternSuffixSkipsBogusPath is AC3: a
// recursive "..."-suffixed pattern names no single directory, so the
// prescription must NOT emit a bogus ".../apicover_named_test.go" path — it
// must fall back to the "EACH package the pattern covers" instruction
// instead. Deleting this branch during the relocation (e.g. reusing the
// ordinary-package path unconditionally) makes this test RED.
func TestGraduationPrescription_PatternSuffixSkipsBogusPath(t *testing.T) {
	got := GraduationPrescription([]string{"./internal/plugins/..."})
	if strings.Contains(got, ".../apicover_named_test.go") {
		t.Errorf("GraduationPrescription([./internal/plugins/...]) = %q, must not emit a bogus \".../apicover_named_test.go\" path for a recursive pattern", got)
	}
	if !strings.Contains(got, "EACH package the pattern covers") {
		t.Errorf("GraduationPrescription([./internal/plugins/...]) = %q, want the \"EACH package the pattern covers\" fallback instruction for a recursive pattern", got)
	}
	if !strings.Contains(got, "append this line to go/.apicover-enforce:  ./internal/plugins/...") {
		t.Errorf("GraduationPrescription([./internal/plugins/...]) = %q, the enforce-append step must still name the pattern verbatim", got)
	}
}

// TestGraduationPrescription_MultiplePackagesEachGetOwnBlock is the semantic
// diversity case: two ungraduated packages must each get their own
// two-step block, not a single block naming only one of them (the class of
// bug a loop-body typo or early-return would introduce).
func TestGraduationPrescription_MultiplePackagesEachGetOwnBlock(t *testing.T) {
	got := GraduationPrescription([]string{"./internal/alpha", "./internal/beta"})
	if !strings.Contains(got, "go/internal/alpha/apicover_named_test.go") {
		t.Errorf("GraduationPrescription([alpha,beta]) = %q, missing alpha's block", got)
	}
	if !strings.Contains(got, "go/internal/beta/apicover_named_test.go") {
		t.Errorf("GraduationPrescription([alpha,beta]) = %q, missing beta's block", got)
	}
}
