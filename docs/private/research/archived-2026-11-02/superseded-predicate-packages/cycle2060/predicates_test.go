//go:build acs

package cycle2060

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	spendPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	measurePkg = "github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
	wavePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/wave"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

	measureDoc = "docs/architecture/packages/internal-wavemeasure.md"
	spendDoc   = "docs/architecture/packages/internal-cyclespend.md"
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
		t.Errorf("RED: %s tests %v fail (exit %d)\n%s", pkg, names, code, acsassert.OutputTail(out, 40))
		return
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass %s", pkg, name)
		}
	}
}

func TestC2060_001_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06(t *testing.T) {
	runNamedTests(t, spendPkg,
		"TestVerdictOf_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06",
		"TestVerdictOf_ACallThatEndsAtTheFailPointCountsBeforeIt",
		"TestVerdictOf_ACycleThatStopsAtTheFailPointWastesNothingAndHasRatio1",
		"TestVerdictOf_ACycleWithNoMeasuredTimeHasNoRatioInsteadOfNaN",
		"TestVerdictOf_ACallWithNoEndTimeIsPlacedByItsTimestamp",
		"TestVerdictOf_ACallWithNoTimeAtAllCountsToTheVerdictButNeverAsWaste",
		"TestVerdictOf_TheMinutesAreTheSameSumAsSpend",
	)
	runNamedTests(t, wavePkg,
		"TestReadCycles_AFloorRejectionAtMinute30OfAFiftyMinuteFailedCycleGivesFailPoint30Waste20Ratio06",
		"TestReadCycles_TheFirstBlockingSignalSetsTheFailPointAmongSeveral",
	)
}

func TestC2060_002_AShippedCycleGivesMinutesToShipAndNoFailPoint(t *testing.T) {
	runNamedTests(t, spendPkg,
		"TestVerdictOf_AShippedCycleHasMinutesToShipAndNoFailPoint",
		"TestVerdictOf_AFailedCycleWithNoBlockingSignalHasNoFailPoint",
	)
	runNamedTests(t, wavePkg,
		"TestReadCycles_AShippedCycleGivesMinutesToShipEvenAfterARejectionThatWasCorrected",
		"TestReadCycles_ACycleWithNoLedgerKeepsAZeroLLMVerdict",
	)
}

func TestC2060_003_WaveStatusListsThePerCycleValuesAndTheWaveMedianFromTheProductionPath(t *testing.T) {
	runNamedTests(t, wavePkg, "TestMeasureFacts_EachCycleCarriesItsLLMVerdictIntoTheCostFacts")
	runNamedTests(t, measurePkg,
		"TestCompute_TheWaveMedianOfLLMMinutesToVerdictAndOfTheFailFastRatio",
		"TestCompute_AnOddCountOfCyclesTakesTheMiddleValue",
		"TestCompute_ACycleWithNoLLMTimeAndACycleWithNoFailPointStayOutOfTheMedians",
		"TestCompute_AWaveWithNoVerdictMinutesHasNullMedians",
		"TestCost_TheJSONFieldNamesAreStable",
	)
	runNamedTests(t, cmdPkg,
		"TestWaveCycleText_ShowsTheLLMMinutesToTheVerdictTheFailPointTheWasteAndTheRatio",
		"TestWaveStatus_TheCostLineShowsTheWaveMedians",
		"TestWaveStatus_JSONCarriesTheLLMVerdictOfEachCycleAndTheWaveMedians",
		"TestWaveStatus_TextListsThePerCycleValuesAndTheWaveMedian",
		"TestWaveStatus_TextPrintsOneCostLine",
	)
}

func lintedDoc(t *testing.T, rel string) (string, []stelint.Finding) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	findings, err := stelint.LintFile(path, stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", rel, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), findings
}

func TestC2060_004_TheDocOfTheMeasureNamesTheVerdictMeasureAndEveryLineAboutItPassesSTE(t *testing.T) {
	for _, rel := range []string{measureDoc, spendDoc} {
		body, findings := lintedDoc(t, rel)
		lines := strings.Split(body, "\n")
		aboutTheMeasure := 0
		for i, line := range lines {
			lower := strings.ToLower(line)
			if !strings.Contains(lower, "fail point") && !strings.Contains(lower, "fail-fast") && !strings.Contains(line, "VerdictOf") {
				continue
			}
			aboutTheMeasure++
			for _, f := range findings {
				if f.Line == i+1 {
					t.Errorf("RED: %s:%d breaks STE (%s): %s", rel, f.Line, f.Rule, f.Message)
				}
			}
		}
		if aboutTheMeasure == 0 {
			t.Errorf("RED: %s has no line about the fail point, the fail-fast ratio or VerdictOf", rel)
		}
	}
	body, _ := lintedDoc(t, measureDoc)
	for _, key := range []string{"median_llm_minutes_to_verdict", "median_fail_fast_ratio", "llm_verdict"} {
		if !strings.Contains(body, key) {
			t.Errorf("RED: %s does not name the JSON key %s", measureDoc, key)
		}
	}
}
