//go:build acs

package cycle1331

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

func TestC1331_001_WarnPrescriptionMintsAddressableLedgerEntry(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestDefectLedger_WarnPrescription",
	)
}

func TestC1331_002_InheritedPrescriptionConsumedByNextCycle(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestReconcile_WarnPrescriptionBlocks",
	)
}

func TestC1331_003_PrescriptionlessWarnBehaviorUnchanged(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestAudit_WarnWithoutPrescription_NoRegression",
	)
}

func TestC1331_004_NewExportViaNewFileInExistingPackageCaught(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate",
	)
}

func TestC1331_005_NewExportViaNewFileNotMisroutedToGraduationGate(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverEnforceChangedDefault_NewExportViaNewFile_NotGraduationGate",
	)
}

func TestC1331_006_GoVetCleanOnTouchedPackages(t *testing.T) {
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/...")
}
