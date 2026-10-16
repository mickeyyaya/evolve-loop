package ciparity

import (
	"strings"
	"testing"
)

func TestGraduationPrescription_EmitsAppendLineAndTestPath(t *testing.T) {
	t.Parallel()
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

func TestGraduationPrescription_EmptyInputReturnsEmptyString(t *testing.T) {
	t.Parallel()
	if got := GraduationPrescription(nil); got != "" {
		t.Errorf("GraduationPrescription(nil) = %q, want \"\" (nothing to prescribe)", got)
	}
	if got := GraduationPrescription([]string{}); got != "" {
		t.Errorf("GraduationPrescription([]) = %q, want \"\" (nothing to prescribe)", got)
	}
}

func TestGraduationPrescription_PatternSuffixSkipsBogusPath(t *testing.T) {
	t.Parallel()
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

func TestGraduationPrescription_MultiplePackagesEachGetOwnBlock(t *testing.T) {
	t.Parallel()
	got := GraduationPrescription([]string{"./internal/alpha", "./internal/beta"})
	if !strings.Contains(got, "go/internal/alpha/apicover_named_test.go") {
		t.Errorf("GraduationPrescription([alpha,beta]) = %q, missing alpha's block", got)
	}
	if !strings.Contains(got, "go/internal/beta/apicover_named_test.go") {
		t.Errorf("GraduationPrescription([alpha,beta]) = %q, missing beta's block", got)
	}
}
