//go:build acs

package cycle1267

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	corePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC1267_001_DirectImportersWidensCorpusToCoveringTestPackages(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestDirectImporters_WidensToReverseImportersIncludingTestOnly",
		"TestDirectImporters_AcceptsBothPatternForms",
		"TestDirectImporters_DeterministicSortedAndDeduped",
		"TestDirectImporters_FailsOpenOnUnusableInput",
		"TestDirectImporters_NoImportersIsNotAnError",
	)
}

func TestC1267_002_DirectImportersReachableFromProduction(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestDirectImporters_ReachableFromProduction",
	)
}

func TestC1267_003_CoveringTestsCapIsLoudNotSilent(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
	)
}

func TestC1267_004_TimeoutOnlySitesNotWidenedToUnion(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_TimeoutOnlyNotWidened",
		"TestTimeoutOnlySites_NotWidenedToUnion",
	)
}

func TestC1267_005_InfraTeardownUnionStillSpelledExactlyOnce(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestInfraTeardownUnion_SpelledExactlyOnce",
		"TestIsInfraTeardownError_UnionSemantics",
		"TestIsTransientBridgeError_StaysTransientOnly",
	)
}

func TestC1267_006_CoveringTestsContractNotRegressedByWidening(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_DedupesAcrossOverlappingPatterns",
		"TestCoveringTests_AcceptsNonRecursivePatternForm",
		"TestCoveringTests_FailsOpenOnUnusableInput",
		"TestCoveringTests_ReachableFromProduction",
	)
}
