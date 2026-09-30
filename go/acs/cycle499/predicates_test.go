//go:build acs

package cycle499

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
const modelcatalogPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC499_001_NewestWinsPicksHighestVersion(t *testing.T) {
	out, code := runGoTest(t, "TestNewestInLineage_PicksHighestVersionAcrossFormats|TestNewestInLineage_RejectsNaiveLexicographicOrdering", modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("newest-wins version comparator is red (exit=%d) — modelquery.NewestInLineage is missing or naively string-compares versions\n%s", code, out)
	}
}

func TestC499_002_NewestWinsHandlesEffortMiniAndUnversioned(t *testing.T) {
	out, code := runGoTest(t, "TestNewestInLineage_IgnoresMiniSuffixWhenComparingVersions|TestNewestInLineage_EffortParentheticalTieFallsBackToInputOrder|TestNewestInLineage_UnversionedFallsBackAndNeverCrashes", modelqueryPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("newest-wins comparator's mini/effort/unversioned handling is red (exit=%d)\n%s", code, out)
	}
}

func TestC499_003_CanonicalTiersIncludesTop(t *testing.T) {
	out, code := runGoTest(t, "TestCanonicalTiersIncludesTop", modelcatalogPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("CanonicalTiers is red (exit=%d) — \"top\" tier not yet added\n%s", code, out)
	}
}

func TestC499_004_HighAliasesToDeepNotTop(t *testing.T) {
	out, code := runGoTest(t, "TestTranslateV1TierKey_HighAliasesToDeep|TestTranslateV1TierKey_TopIsNotConflatedWithHigh|TestTranslateV1TierKey_PreexistingMappingsUnchanged", bridgePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("translateV1TierKey \"high\" alias is red (exit=%d)\n%s", code, out)
	}
}
