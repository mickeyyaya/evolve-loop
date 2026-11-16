//go:build acs

package cycle2428

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	spendPackage   = "./internal/cyclespend"
	wavePackage    = "./internal/wave"
	measurePackage = "./internal/wavemeasure"
	cmdPackage     = "./cmd/evolve"
	enrollmentFile = "go/.apicover-enforce"
)

var (
	verdictTests = []string{
		"TestVerdict_CountsHostMinutes",
		"TestVerdict_HostMinutesOfAShippedCycleAreCountedAndLeaveNoFailPoint",
		"TestVerdict_NoGateSpansLeaveTheHostMinutesAtZeroAndTheVerdictUnchanged",
		"TestVerdict_ASpanThatEndsBeforeItStartsCountsZeroHostMinutes",
		"TestGateSpan_MinutesIsTheSpanLengthAndNeverNegative",
	}
	samplerTests = []string{
		"TestSampler_RecordsLoadForEachGateSpan",
		"TestSampler_AGateThatFailsStillRecordsItsSpanAndLoadAndReturnsItsError",
		"TestSampler_FlushAppendsTheSpansToTheWorkspaceLedgerAndEmptiesTheBuffer",
		"TestSampler_FlushOfNoSpansWritesNoLedger",
		"TestReadGateSpans_AMissingLedgerIsNoSpansAndNoError",
		"TestReadGateSpans_AMalformedLineKeepsTheGoodSpansAndNamesTheLedgerInTheError",
	}
	waveReadTests = []string{
		"TestReadCycles_TheGateSpansOfACycleGiveItsHostMinutesBesideItsLLMMinutes",
		"TestReadCycles_ACycleWithNoGateSpansHasZeroHostMinutes",
		"TestReadCycles_AMalformedGateSpanLedgerIsAWarningAndTheLLMVerdictStaysWhole",
	}
	costTests = []string{
		"TestCompute_HostMinutesPerShippedCycleAndTheirMedianSitBesideTheLLMMinutes",
		"TestCompute_AWaveWithNoHostMinutesHasNoHostFigures",
		"TestCompute_AWaveWithHostMinutesAndNoShipHasAMedianButNoPerShipFigure",
	}
	printerTests = []string{
		"TestWaveCycleText_ShowsTheHostMinutesBesideTheLLMMinutes",
		"TestWaveStatus_TheCostLineShowsTheHostMinutesBesideTheLLMMinutes",
		"TestWaveStatus_JSONCarriesTheHostMinutesOfEachCycleAndTheWaveMedian",
		"TestWaveStatus_TextShowsTheHostMinutesBesideTheLLMMinutes",
	}
	preservedSpendTests = []string{
		"TestVerdictOf_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06",
		"TestVerdictOf_AShippedCycleHasMinutesToShipAndNoFailPoint",
	}
	preservedPrinterTests = []string{
		"TestWaveCycleText_ShowsTheLLMMinutesToTheVerdictTheFailPointTheWasteAndTheRatio",
		"TestWaveStatus_TheCostLineShowsTheWaveMedians",
		"TestWaveStatus_TextPrintsOneCostLine",
		"TestWaveStatus_TextListsThePerCycleValuesAndTheWaveMedian",
	}
)

func requireGreen(t *testing.T, pkg string, names []string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-timeout", "540s", "-run", pattern, pkg)
	all := out + errOut
	if code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", pkg, code, acsassert.OutputTail(all, 40))
	}
	for _, name := range names {
		if !strings.Contains(all, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2428_001_VerdictCountsHostMinutesBesideLLMMinutes(t *testing.T) {
	requireGreen(t, spendPackage, verdictTests)
}

func TestC2428_002_SamplerRecordsLoadForEachGateSpan(t *testing.T) {
	requireGreen(t, spendPackage, samplerTests)
}

func TestC2428_003_WaveReadingsCarryHostMinutesFromTheWorkspaceGateSpans(t *testing.T) {
	requireGreen(t, wavePackage, waveReadTests)
}

func TestC2428_004_WaveCostShowsHostMinutesPerShippedCycleAndTheirMedian(t *testing.T) {
	requireGreen(t, measurePackage, costTests)
}

func TestC2428_005_WaveStatusCommandPrintsAndEncodesHostMinutesBesideLLMMinutes(t *testing.T) {
	requireGreen(t, cmdPackage, printerTests)
}

func TestC2428_006_LLMOnlyReadingsStayUnchangedWhenNoGateSpanExists(t *testing.T) {
	requireGreen(t, spendPackage, preservedSpendTests)
	requireGreen(t, cmdPackage, preservedPrinterTests)
}

func TestC2428_007_EveryNewCyclespendExportIsNamedAndCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	if !acsassert.FileContains(t, filepath.Join(root, enrollmentFile), "\n"+spendPackage+"\n") {
		t.Fatalf("RED: %s is not enrolled in %s", spendPackage, enrollmentFile)
	}
	work := t.TempDir()
	profile := filepath.Join(work, "cover.out")
	if out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile", profile, spendPackage); code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", spendPackage, code, acsassert.OutputTail(out+errOut, 30))
	}
	funcs, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func", profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exited %d: %v\n%s", code, err, errOut)
	}
	funcFile := filepath.Join(work, "cover.func.txt")
	if writeErr := os.WriteFile(funcFile, []byte(funcs), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	var report strings.Builder
	rc, runErr := apicover.Run(context.Background(), apicover.Config{Dirs: []string{filepath.Join(goDir, "internal", "cyclespend")}, CoverPath: funcFile, Enforce: true}, &report)
	if runErr != nil || rc != 0 {
		t.Errorf("RED: apicover -enforce on internal/cyclespend exited %d: %v\n%s", rc, runErr, report.String())
	}
}
