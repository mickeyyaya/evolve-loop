//go:build acs

package cycle1329

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing, failing, or a build-compile error). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1329_001_CiparityGraduationPrescriptionExportedAndCorrect(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/ciparity",
		"TestGraduationPrescription_EmitsAppendLineAndTestPath",
		"TestGraduationPrescription_EmptyInputReturnsEmptyString",
		"TestGraduationPrescription_PatternSuffixSkipsBogusPath",
		"TestGraduationPrescription_MultiplePackagesEachGetOwnBlock",
	)
}

func TestC1329_002_AuditOffenderCarriesPrescriptiveFix(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix",
	)
}

func TestC1329_003_BuildSeamRegressionStaysGreenThroughRelocation(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/core",
		"TestBuildGraduationCheck",
		"TestRecordAndBranch_BuildGraduationGuardAborts",
		"TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds",
	)
}

func assertVetClean(t *testing.T, pkg string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", pkg)
	if code == -1 {
		t.Fatalf("go vet failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	if code != 0 {
		t.Errorf("go vet %s exited %d, want 0 clean\nstdout:\n%s\nstderr:\n%s", pkg, code, stdout, stderr)
	}
}

func TestC1329_004_GoVetCleanOnTouchedPackages(t *testing.T) {
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/ciparity/...")
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/core/...")
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/...")
}
