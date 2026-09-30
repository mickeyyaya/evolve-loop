//go:build acs

package cycle1034

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(unlanded, failing, or the package failed to compile). exit=%d\ncombined output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1034_001_PreClassBucketsFromRealArtifacts(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_PreClassBucketsFromRealArtifacts")
}

func TestC1034_002_FingerprintStableAndPhaseComposed(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_FingerprintComposition")
}

func TestC1034_003_RecurrenceReadThroughLedger(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_RecurrenceFromLedger")
}

func TestC1034_004_MissingArtifactsDegradeToUnknown(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_MissingArtifactsDegradeToUnknown")
}

func TestC1034_005_DigestWrittenAsValidJSON(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_WritesDigestArtifact")
}

func TestC1034_006_RetroFailsLoudWithoutValidDisposition(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_RetroFailsLoudWithoutValidDisposition")
}

func TestC1034_007_FingerprintCrossCheckedAgainstDigest(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_CrossChecksFingerprintAgainstDigest")
}

func TestC1034_008_RejectsInvalidEnums(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_RejectsInvalidEnums")
}

func TestC1034_009_SalvagePointerRequiredWhenValue(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestDispositionGate_SalvagePointerRequiredWhenValue")
}

func TestC1034_010_GateWiredIntoRetroCompletion(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDispositionGate_WiredIntoRetroCompletion",
		"TestFinalizeRetroCompletion_SeamContract")
}
