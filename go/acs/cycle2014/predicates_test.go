//go:build acs

package cycle2014

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ladderPkg = "github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	cmdPkg    = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runNamedTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", pkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("RED: %s tests %v fail (exit %d)\n%s", pkg, names, code, tail(out))
		return
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass %s", pkg, name)
		}
	}
}

func tail(s string) string {
	if len(s) > 1500 {
		return s[len(s)-1500:]
	}
	return s
}

func TestC2014_001_AggregatorCountsTriesFixesAndTokensPerRungAndClass(t *testing.T) {
	runNamedTests(t, ladderPkg,
		"TestLadderReport_ExactCountsPerRungAndClass",
		"TestLadderReport_RunsAreIndependent",
		"TestLadderReport_RecordsWithoutARungAreLeftOut",
		"TestLadderReport_LoadRunsReadsEachRunDirLedgerInNameOrder",
	)
}

func TestC2014_002_VerbPrintsExactCountsAsTextAndJSON(t *testing.T) {
	runNamedTests(t, cmdPkg,
		"TestLadderReport_VerbPrintsExactCountsPerRungAndClass",
		"TestLadderReport_VerbJSONCarriesTheSameCounts",
		"TestLadderReport_RegistrySummaryNamesTheVerbAndItsFlags",
	)
}

func TestC2014_003_VerbWritesNothing(t *testing.T) {
	runNamedTests(t, cmdPkg, "TestLadderReport_VerbWritesNothing")
}

func TestC2014_004_RejectsWhatIsNotAFixAndFailsLoudlyOnBadInput(t *testing.T) {
	runNamedTests(t, ladderPkg,
		"TestLadderReport_FixRequiresARetryThatPassed",
		"TestLadderReport_LoadRunsFailsLoudlyOnAnAbsentRunsDir",
	)
	runNamedTests(t, cmdPkg,
		"TestLadderReport_VerbExitCodes",
		"TestLadderReport_VerbEmptyRunsDirPrintsOnlyTheHeader",
	)
}
