//go:build acs

package cycle2176

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPkg          = "./cmd/evolve"
	modelcatalogPkg = "./internal/modelcatalog"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requireTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s exit=%d err=%v\n%s\n%s", pkg, code, err, acsassert.OutputTail(out, 40), acsassert.OutputTail(errOut, 20))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func coverFunc(t *testing.T, pkg, runPattern string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-run", runPattern, "-coverprofile="+profile, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test -cover %s exit=%d err=%v\n%s\n%s", pkg, code, err, acsassert.OutputTail(out, 40), acsassert.OutputTail(errOut, 20))
	}
	funcOut, funcErr, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v\n%s", code, err, funcErr)
	}
	return funcOut
}

func requireFullCoverage(t *testing.T, coverOutput string, funcs ...string) {
	t.Helper()
	for _, name := range funcs {
		pct, err := acsassert.FuncCoverage(coverOutput, name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if pct != 100.0 {
			t.Errorf("%s coverage = %.1f%%, want 100.0%%", name, pct)
		}
	}
}

func TestC2176_001_ADegradedFamilyKeepsItsLivePickAndEmitsTheRegisteredEvent(t *testing.T) {
	requireTestsPass(t, modelcatalogPkg,
		"TestRefreshDegradedEvent_NamesTheFamilyAndItsFallbackReasonAsARegisteredConfigWarning",
		"TestRefreshDegradedEvent_PassesTheCenterWithoutBeingRewritten",
		"TestRefreshDegradedEvent_AnEmptyFallbackReasonStillGivesAReasonAndOmitsNothingElse",
		"TestKeepLivePicks_AFamilyWhoseListFailedKeepsItsPriorLivePick",
	)
	requireTestsPass(t, cmdPkg,
		"TestCatalogDegraded_EnforceEmitsOneEventPerFamilyWhoseListFailed",
		"TestRunStagedCatalogRefresh_EnforceKeepsThePriorLivePickOfAFamilyWhoseListFailed",
	)
}

func TestC2176_002_NoEventWhenNothingDegradedAtAnyStage(t *testing.T) {
	requireTestsPass(t, cmdPkg,
		"TestCatalogDegraded_NoEventWhenEveryFamilyListedLive",
		"TestCatalogDegraded_NoEventForAFamilyThatNeverHadALivePick",
		"TestCatalogDegraded_ShadowAndZeroLiveEmitNothingAndWriteNoLiveCatalog",
	)
}

func TestC2176_003_TheLoopBootRefreshIsWiredFromTheCompositionRootAndRunsBeforeThePreflight(t *testing.T) {
	requireTestsPass(t, cmdPkg,
		"TestRefreshCatalogAtLoopBoot_RunsTheRefresherOnce",
		"TestRefreshCatalogAtLoopBoot_AnErrorWarnsAndDoesNotAbortTheBoot",
		"TestRefreshCatalogAtLoopBoot_ANilRefresherIsSkipped",
		"TestRunLoopBatch_RefreshesTheCatalogOnceBeforeThePreflight",
		"TestWireOrchestratorDeps_TheCatalogRefresherIsWiredAndTheTTLStopsItOnAFreshCatalog",
	)
}

func TestC2176_004_TheTTLGatesEveryRefreshAndAFreshCatalogIsNeverRefreshed(t *testing.T) {
	requireTestsPass(t, cmdPkg,
		"TestCatalogDegraded_AFreshCatalogIsNotRefreshedAtEitherStage",
		"TestWireOrchestratorDeps_TheCatalogRefresherIsWiredAndTheTTLStopsItOnAFreshCatalog",
	)
}

func TestC2176_005_AZeroLiveRefreshWritesNeitherCatalogAndKeepsFetchedAt(t *testing.T) {
	requireTestsPass(t, cmdPkg,
		"TestCatalogDegraded_ZeroLiveKeepsFetchedAtSoTheNextCycleStartRetries",
		"TestRunStagedCatalogRefresh_ZeroLiveClassificationsWriteNeitherCatalog",
	)
}

func TestC2176_006_BothCallersUseTheOneModelcatalogPredicate(t *testing.T) {
	requireTestsPass(t, modelcatalogPkg, "TestCatalog_LiveCountCountsOnlyTheLiveEntries")
	requireTestsPass(t, cmdPkg, "TestCatalogDegraded_CountLiveAndFallbackReadsThePredicateOfModelcatalog")
	cmdDir := filepath.Join(goDir(t), "cmd", "evolve")
	for file, fn := range map[string]string{"cmd_models.go": "countLiveAndFallback", "cmd_models_live.go": "runStagedCatalogRefresh"} {
		path := filepath.Join(cmdDir, file)
		if n, err := acsassert.CountInGoFunc(path, fn, "SourceLive"); err != nil || n != 0 {
			t.Errorf("%s in %s counts Source == live itself (n=%d, err=%v): the predicate must live only in modelcatalog", fn, file, n, err)
		}
		if n, err := acsassert.CountInGoFunc(path, fn, "LiveCount"); err != nil || n < 1 {
			t.Errorf("%s in %s never calls LiveCount (n=%d, err=%v)", fn, file, n, err)
		}
	}
}

func TestC2176_007_ChangedModelcatalogFunctionsAreAtFullCoverage(t *testing.T) {
	out := coverFunc(t, modelcatalogPkg, "^(TestRefreshDegradedEvent_|TestKeepLivePicks_|TestCatalog_LiveCount)")
	requireFullCoverage(t, out, "RefreshDegradedEvent", "KeepLivePicks", "LiveCount")
}

func TestC2176_008_ChangedCmdEvolveFunctionsAreAtFullCoverage(t *testing.T) {
	out := coverFunc(t, cmdPkg, "^(TestRunStagedCatalogRefresh_|TestCatalogDegraded_|TestMakeCatalogRefresher_|TestRefreshCatalogAtLoopBoot_)")
	requireFullCoverage(t, out, "makeCatalogRefresher", "runStagedCatalogRefresh", "commitKeepingLivePicks", "refreshCatalogAtLoopBoot", "countLiveAndFallback")
}

func TestC2176_009_ThePlanC3RowAndThePackageNoteRecordTheChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	plan := filepath.Join(root, "docs", "plans", "model-currency-2026-10.md")
	note := filepath.Join(root, "docs", "architecture", "packages", "internal-modelcatalog.md")
	// acs-predicate: config-check
	acsassert.AllOf(t,
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, plan, "CONFIG_CATALOG_REFRESH_DEGRADED") },
		func(tb acsassert.TB) bool {
			return acsassert.FileNotContains(tb, plan, "The refresh does not run at loop boot.")
		},
		func(tb acsassert.TB) bool {
			return acsassert.FileNotContains(tb, plan, "at **loop boot** (open, not built)")
		},
		func(tb acsassert.TB) bool {
			return acsassert.FileNotContains(tb, plan, "a boot-time refresh is TTL-gated (open, not built)")
		},
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, note, "RefreshDegradedEvent") },
		func(tb acsassert.TB) bool { return acsassert.FileContains(tb, note, "CONFIG_CATALOG_REFRESH_DEGRADED") },
	)
}

func TestC2176_010_TheCompositionRootGivesTheRefresherTheSignalCenterEmitter(t *testing.T) {
	path := filepath.Join(goDir(t), "cmd", "evolve", "cmd_cycle.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(path, "makeCatalogRefresher(", "signals.Emit") {
		t.Errorf("%s never passes signals.Emit to makeCatalogRefresher: the degraded event would reach no Center", path)
	}
}
