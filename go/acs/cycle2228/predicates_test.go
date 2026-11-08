//go:build acs

package cycle2228

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const repocontractPkg = "github.com/mickeyyaya/evolve-loop/go/internal/repocontract"

func assertNamedTestsPass(t *testing.T, names ...string) {
	t.Helper()
	assertNamedTestsPassWithTags(t, "", names...)
}

func assertNamedTestsPassWithTags(t *testing.T, tags string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	args := []string{"test"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "-run", pattern, "-v", "-count=1", repocontractPkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", repocontractPkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("test %s did NOT pass in %s\n%s", name, repocontractPkg, acsassert.OutputTail(out, 40))
		}
	}
	if code != 0 {
		t.Errorf("go test exited %d", code)
	}
}

func TestC2228_001_EachFileParsesOnceForEachBinary(t *testing.T) {
	assertNamedTestsPass(t,
		"TestStreamParse_EachBoundFileParsesOnceAcrossBothScanners",
		"TestStreamParse_TheCounterCountsEveryParseOfAFile",
		"TestStreamParse_GroupsFilesByDirectoryAndKeepsFirstSeenOrder",
		"TestStreamParse_AParseErrorNamesTheFileAndStopsTheWalk",
		"TestStreamParse_AVisitorErrorStopsTheWalk",
		"TestStreamParse_TheTwoScannerTestsUseTheSharedScanAndParseNothingThemselves",
		"TestNoTestWritesIntoTheRealTree",
		"TestPackages_HoldEveryTestThatReadsTheWholeTree",
	)
}

func TestC2228_002_TheRepocontractScanPeakHeapIsBelow400MB(t *testing.T) {
	assertNamedTestsPass(t,
		"TestRepoContract_PeakHeapUnder400MB",
		"TestRepoContract_ScanSamplesTheHeapWhileAUnitIsLive",
		"TestStreamParse_ReleasesEachUnitBeforeTheNextOne",
		"TestStreamParse_ARetainingVisitorIsSeenToHoldItsUnits",
	)
}

func TestC2228_003_GoldenRunOfOldAndNewScannerGivesTheSameFindings(t *testing.T) {
	assertNamedTestsPassWithTags(t, "acs",
		"TestRepoContract_GoldenParity",
		"TestRepoContract_GoldenParityDetectsADivergence",
		"TestStreamParse_ScanFlagsAWriteAndNamesTheReadingTest",
	)
}

func listedTests(t *testing.T, tags string) string {
	t.Helper()
	args := []string{"test"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "-list", ".*", repocontractPkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code != 0 {
		t.Fatalf("go test -list (tags %q) exited %d: %v\n%s", tags, code, err, acsassert.OutputTail(stdout+stderr, 40))
	}
	return stdout
}

func TestC2228_004_TheDefaultBinaryHoldsNoSecondWholeTreeParse(t *testing.T) {
	golden := []string{"TestRepoContract_GoldenParity", "TestRepoContract_GoldenParityDetectsADivergence"}
	defaultList := strings.Fields(listedTests(t, ""))
	acsList := strings.Fields(listedTests(t, "acs"))
	for _, name := range golden {
		if slices.Contains(defaultList, name) {
			t.Errorf("%s is in the default repocontract binary: it re-parses every bound file outside the parse counter on each CI run", name)
		}
		if !slices.Contains(acsList, name) {
			t.Errorf("%s is missing under -tags acs: the golden comparison of the old and the new scanner must still exist", name)
		}
	}
	for _, name := range []string{"TestRepoContract_PeakHeapUnder400MB", "TestStreamParse_EachBoundFileParsesOnceAcrossBothScanners"} {
		if !slices.Contains(defaultList, name) {
			t.Errorf("%s must stay in the default binary", name)
		}
	}
}
