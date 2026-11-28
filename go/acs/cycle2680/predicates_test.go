//go:build acs

package cycle2680

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePackage   = "cmd/evolve"
	baselineRel        = "cmd/evolve/testdata/testaudit-baseline.json"
	baselineRowsBefore = 182
)

var sliceRows = []testaudit.Finding{
	{Package: cmdEvolvePackage, Test: "TestBinaryAnswersSubcommandDep_OnlyAnUnknownCommandIsARefusal", Rule: "unverified-fake"},
	{Package: cmdEvolvePackage, Test: "TestBootCLIUpdate_ASmokeFailureWhileTheFamilyIsDrainedWarnsAndDoesNotHalt", Rule: "mock-call-order"},
	{Package: cmdEvolvePackage, Test: "TestBootCLIUpdate_ASmokeFailureWithOnlyArgonDrainedStillHaltsAndNamesTheWindow", Rule: "mock-call-order"},
	{Package: cmdEvolvePackage, Test: "TestCenterlessConfigLoadSitesArePinned", Rule: "asserts-own-setup"},
	{Package: cmdEvolvePackage, Test: "TestCycleRunJourney_AFleetLaneWithAQuotaFamilyAndALoggedOutFamilyReleasesItsClaimAndNamesBoth", Rule: "mock-call-order"},
	{Package: cmdEvolvePackage, Test: "TestE2E_M4LedgerEntryShapeMatch", Rule: "no-assertion"},
	{Package: cmdEvolvePackage, Test: "TestJourneyLaneHelper", Rule: "no-assertion"},
	{Package: cmdEvolvePackage, Test: "TestLoopDetachHelperProcess", Rule: "no-assertion"},
	{Package: cmdEvolvePackage, Test: "TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos", Rule: "mock-call-order"},
	{Package: cmdEvolvePackage, Test: "TestRecordAbsorbedFail_MissingStateIsSoftWarn", Rule: "no-assertion"},
	{Package: cmdEvolvePackage, Test: "TestSyncMain_AFailedReplayCommitExitsOneAndNamesTheReplay", Rule: "unverified-fake"},
	{Package: cmdEvolvePackage, Test: "TestSyncMain_AQuarantineMoveThatCannotBeRecordedRefusesBeforeTheMerge", Rule: "unverified-fake"},
	{Package: cmdEvolvePackage, Test: "TestSyncMain_RefusesToMergeWhenTheStampsCannotLand", Rule: "unverified-fake"},
}

func readBaselineRows(t *testing.T) []testaudit.Finding {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.GoModuleDir(t), filepath.FromSlash(baselineRel)))
	if err != nil {
		t.Fatalf("read the cmd/evolve baseline: %v", err)
	}
	var shard struct {
		Rows []testaudit.Finding `json:"rows"`
	}
	if err := json.Unmarshal(raw, &shard); err != nil {
		t.Fatalf("parse the cmd/evolve baseline: %v", err)
	}
	return shard.Rows
}

func analyzeModule(t *testing.T) (testaudit.Config, []testaudit.Finding) {
	t.Helper()
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	found, err := testaudit.AnalyzeTree(acsassert.GoModuleDir(t), cfg)
	if err != nil {
		t.Fatalf("testaudit.AnalyzeTree: %v", err)
	}
	return cfg, found
}

func TestC2680_001_TheBaselineDropsEveryRowOfTheSlice(t *testing.T) {
	have := map[testaudit.Finding]bool{}
	for _, row := range readBaselineRows(t) {
		have[row] = true
	}
	for _, row := range sliceRows {
		if have[row] {
			t.Errorf("baseline still holds %s %s %s", row.Package, row.Test, row.Rule)
		}
	}
}

func TestC2680_002_TheAnalyzerNoLongerReportsAnyFindingOfTheSlice(t *testing.T) {
	_, found := analyzeModule(t)
	have := map[testaudit.Finding]bool{}
	for _, f := range found {
		have[f] = true
	}
	for _, row := range sliceRows {
		if have[row] {
			t.Errorf("the analyzer still reports %s %s %s", row.Package, row.Test, row.Rule)
		}
	}
}

func TestC2680_003_TheBaselineAndTheAnalyzerAgreeForCmdEvolveAndTheBaselineNeverGrows(t *testing.T) {
	cfg, _ := analyzeModule(t)
	err := testaudit.CheckTree(acsassert.GoModuleDir(t), cfg)
	for _, problem := range testaudit.Problems(err) {
		if row, ok := problem.(*testaudit.RowError); ok && row.Finding.Package == cmdEvolvePackage {
			t.Errorf("%v", row)
		}
	}
	if rows := readBaselineRows(t); len(rows) > baselineRowsBefore-len(sliceRows) {
		t.Errorf("baseline holds %d rows, want at most %d: a rewrite may only shrink it", len(rows), baselineRowsBefore-len(sliceRows))
	}
}

type mutant struct {
	file     string
	old, new string
	tags     string
	tests    []string
}

func mutantFailsTheTests(t *testing.T, m mutant) {
	t.Helper()
	goDir := acsassert.GoModuleDir(t)
	target := filepath.Join(goDir, filepath.FromSlash(m.file))
	src, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if n := strings.Count(string(src), m.old); n != 1 {
		t.Fatalf("mutation site in %s matched %d times, want exactly 1: %q", m.file, n, m.old)
	}
	work := t.TempDir()
	mutated := filepath.Join(work, filepath.Base(m.file))
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {target: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(work, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code, _ := acsassert.SubprocessOutputIn(goDir, "go", "test", "-count=1", "-tags", m.tags, "-overlay", overlayPath,
		"-run", "^("+strings.Join(m.tests, "|")+")$", "./cmd/evolve")
	if code == 0 {
		t.Fatalf("the mutant %q -> %q in %s passes %v: each rewritten test must fail on it", m.old, m.new, m.file, m.tests)
	}
	for _, name := range m.tests {
		if !strings.Contains(out+errOut, "--- FAIL: "+name) {
			t.Errorf("%s did not fail on the mutant in %s:\n%s%s", name, m.file, out, errOut)
		}
	}
}

func TestC2680_004_AbsorbedFailMissingStateTestFailsWhenTheMissingStateWarns(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_loop_control.go",
		old:   "!errors.Is(err, failurelog.ErrStateMissing)",
		new:   "!errors.Is(err, errors.ErrUnsupported)",
		tests: []string{"TestRecordAbsorbedFail_MissingStateIsSoftWarn"},
	})
}

func TestC2680_005_ResupplyTestFailsWhenTheResupplyIgnoresTheHeldTodos(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_loop_pool.go",
		old:   "plan(depth+len(held))",
		new:   "plan(depth)",
		tests: []string{"TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos"},
	})
}

func TestC2680_006_SubcommandDepTestFailsWhenTheRefusalIsNeverRecognized(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_release_verify_clis.go",
		tags:  "integration",
		old:   `fmt.Sprintf("unknown command %q", sub)`,
		new:   `fmt.Sprintf("unknown command %q", sub+"x")`,
		tests: []string{"TestBinaryAnswersSubcommandDep_OnlyAnUnknownCommandIsARefusal"},
	})
}

func TestC2680_007_DrainedFamilyTestFailsWhenADrainedFamilyReadsAsABrokenSmoke(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "internal/cliupdate/cliupdate.go",
		old:   "return r.with(StatusQuotaExhausted, \"smoke failed, and a usage query verified the cause: \"",
		new:   "return r.with(StatusSmokeFailed, \"smoke failed, and a usage query verified the cause: \"",
		tests: []string{"TestBootCLIUpdate_ASmokeFailureWhileTheFamilyIsDrainedWarnsAndDoesNotHalt"},
	})
}

func TestC2680_008_ArgonWindowTestFailsWhenNoSmokeFailureHalts(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_loop_cliupdate.go",
		old:   "if !smokeBypassable(deps, cfg.ProjectRoot, failed) {",
		new:   "if false {",
		tests: []string{"TestBootCLIUpdate_ASmokeFailureWithOnlyArgonDrainedStillHaltsAndNamesTheWindow"},
	})
}

func TestC2680_009_SyncMainReplayTestFailsWhenAFailedReplayExitsZero(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_syncmain.go",
		old:   "the rest are in the tree, uncommitted\\n\", err)\n\t\treturn 1",
		new:   "the rest are in the tree, uncommitted\\n\", err)\n\t\treturn 0",
		tests: []string{"TestSyncMain_AFailedReplayCommitExitsOneAndNamesTheReplay"},
	})
}

func TestC2680_010_SyncMainQuarantineTestFailsWhenAFailedMoveRecordDoesNotRefuse(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_syncmain.go",
		old:   "quarantine moves failed: %v\\n\", err)\n\t\treturn nil, 1",
		new:   "quarantine moves failed: %v\\n\", err)\n\t\treturn partition.Stamps, 0",
		tests: []string{"TestSyncMain_AQuarantineMoveThatCannotBeRecordedRefusesBeforeTheMerge"},
	})
}

func TestC2680_011_SyncMainLandingTestFailsWhenAFailedStampCommitIsIgnored(t *testing.T) {
	mutantFailsTheTests(t, mutant{
		file:  "cmd/evolve/cmd_syncmain.go",
		old:   "err = plan.CommitKept(ctx, g)",
		new:   "_ = plan.CommitKept(ctx, g)",
		tests: []string{"TestSyncMain_RefusesToMergeWhenTheStampsCannotLand"},
	})
}

func TestC2680_012_TheRewrittenTestsAndTheHelperConsumersStillPass(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	names := []string{
		"TestBinaryAnswersSubcommandDep_OnlyAnUnknownCommandIsARefusal",
		"TestBootCLIUpdate_ASmokeFailureWhileTheFamilyIsDrainedWarnsAndDoesNotHalt",
		"TestBootCLIUpdate_ASmokeFailureWithOnlyArgonDrainedStillHaltsAndNamesTheWindow",
		"TestCenterlessConfigLoadSitesArePinned",
		"TestCycleRunJourney_AFleetLaneWithAQuotaFamilyAndALoggedOutFamilyReleasesItsClaimAndNamesBoth",
		"TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos",
		"TestRecordAbsorbedFail_MissingStateIsSoftWarn",
		"TestSyncMain_AFailedReplayCommitExitsOneAndNamesTheReplay",
		"TestSyncMain_AQuarantineMoveThatCannotBeRecordedRefusesBeforeTheMerge",
		"TestSyncMain_RefusesToMergeWhenTheStampsCannotLand",
		"TestLoopDetach",
	}
	out, errOut, code, _ := acsassert.SubprocessOutputIn(goDir, "go", "test", "-count=1", "-tags", "integration",
		"-run", "^("+strings.Join(names, "|")+")", "./cmd/evolve")
	if code != 0 {
		t.Errorf("the rewritten tests and the detach consumers must pass (exit %d):\n%s%s", code, out, errOut)
	}
}

func TestC2680_013_TheJourneyConsumersOfTheLaneHelperStillPass(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	out, errOut, code, _ := acsassert.SubprocessOutputIn(goDir, "go", "test", "-count=1", "-tags", "integration",
		"-run", "^TestJourney_", "./cmd/evolve")
	if code != 0 {
		t.Errorf("the journey tests that run the lane helper must pass (exit %d):\n%s%s", code, out, errOut)
	}
}
