//go:build acs

package cycle2564

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const landcmdPackage = "./internal/cli/landcmd"

var (
	buildOnce   sync.Once
	builtBinary string
	buildOutput string
	buildCode   int
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-cycle2564-")
		if err != nil {
			buildCode = -1
			buildOutput = err.Error()
			return
		}
		builtBinary = filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", builtBinary, "./cmd/evolve")
		buildCode = code
		buildOutput = stdout + stderr
	})
	if buildCode != 0 {
		t.Fatalf("go build ./cmd/evolve exit %d\n%s", buildCode, acsassert.OutputTail(buildOutput, 40))
	}
	return builtBinary
}

func mainDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
}

func parsedFiles(t *testing.T, dir string, keep func(name string) bool) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || !keep(e.Name()) {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files[e.Name()] = file
	}
	return files
}

func isProduction(name string) bool { return !strings.HasSuffix(name, "_test.go") }

func isTest(name string) bool { return strings.HasSuffix(name, "_test.go") }

func landVerbFile(name string) bool {
	for _, group := range []string{"cmd_pr", "cmd_ci"} {
		if name == group+".go" || name == group+"_test.go" || strings.HasPrefix(name, group+"_") {
			return true
		}
	}
	return false
}

func nonTestLandFilesInMain(t *testing.T) []string {
	t.Helper()
	var kept []string
	for name := range parsedFiles(t, mainDir(t), func(n string) bool { return isProduction(n) && landVerbFile(n) }) {
		kept = append(kept, name)
	}
	slices.Sort(kept)
	return kept
}

func funcDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func landcmdCall(fn *ast.FuncDecl, callee string) *ast.CallExpr {
	var found *ast.CallExpr
	if fn == nil {
		return nil
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == callee {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "landcmd" {
				found = call
			}
		}
		return true
	})
	return found
}

func TestC2564_001_TestLandCmdRunPassesInTheNewPackage(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: landcmdPackage,
		Pattern: "^TestLandCmdRun$",
		Names:   []string{"TestLandCmdRun"},
	})
}

func TestC2564_002_OnlyTheTwoShimsRemainAmongTheNonTestPrAndCiFilesOfMain(t *testing.T) {
	got := nonTestLandFilesInMain(t)
	if !slices.Equal(got, []string{"cmd_ci.go", "cmd_pr.go"}) {
		t.Fatalf("non-test cmd_pr*.go and cmd_ci*.go files in go/cmd/evolve = %v, want exactly [cmd_ci.go cmd_pr.go]", got)
	}
	for _, shim := range got {
		path := filepath.Join(mainDir(t), shim)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if lines := strings.Count(string(source), "\n"); lines > 40 {
			t.Errorf("%s has %d lines; a shim is a thin hand-over, not the verb group", shim, lines)
		}
		if !callsLandcmd(parsedFiles(t, mainDir(t), func(n string) bool { return n == shim })[shim]) {
			t.Errorf("%s never calls into the landcmd package", shim)
		}
	}
}

func callsLandcmd(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "landcmd" {
				found = true
			}
		}
		return !found
	})
	return found
}

func TestC2564_003_TheRegistryVerbsReachLandcmdThroughTheShimsWithAWiredDepsValue(t *testing.T) {
	registry := parsedFiles(t, mainDir(t), func(n string) bool { return n == "registry.go" })["registry.go"]
	handlers := map[string]string{}
	ast.Inspect(registry, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		var verb, run string
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, _ := kv.Key.(*ast.Ident)
			switch {
			case key != nil && key.Name == "Name":
				if basic, ok := kv.Value.(*ast.BasicLit); ok {
					verb = strings.Trim(basic.Value, `"`)
				}
			case key != nil && key.Name == "Run":
				if id, ok := kv.Value.(*ast.Ident); ok {
					run = id.Name
				}
			}
		}
		if verb != "" && run != "" {
			handlers[verb] = run
		}
		return true
	})
	files := parsedFiles(t, mainDir(t), func(n string) bool { return n == "cmd_pr.go" || n == "cmd_ci.go" })
	for verb, spec := range map[string]struct{ file, callee string }{
		"pr": {"cmd_pr.go", "RunPR"},
		"ci": {"cmd_ci.go", "RunCI"},
	} {
		handler := handlers[verb]
		if handler == "" {
			t.Errorf("registry.go has no handler for verb %q", verb)
			continue
		}
		call := landcmdCall(funcDecl(files[spec.file], handler), spec.callee)
		if call == nil {
			t.Errorf("registry verb %q runs %s, which does not call landcmd.%s in %s", verb, handler, spec.callee, spec.file)
			continue
		}
		last := call.Args[len(call.Args)-1]
		if lit, ok := last.(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
			t.Errorf("%s passes an empty landcmd.Deps to landcmd.%s: the Deps seam is unwired", handler, spec.callee)
		}
	}
}

func TestC2564_004_PrAndCiBehaveTheSameThroughTheProductionBinary(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"pr with no verb is a usage error", []string{"pr"}, 10, "", "usage: evolve pr merge <n>..."},
		{"pr merge needs a number", []string{"pr", "merge"}, 10, "", "evolve pr merge: name at least one PR number"},
		{"pr merge refuses a non-numeric number", []string{"pr", "merge", "abc"}, 10, "", `PR operand "abc" must be a distinct positive number`},
		{"pr window needs a number or a file", []string{"pr", "window"}, 10, "", "evolve pr window: name a PR number or --files-from"},
		{"pr boundary refuses an operand", []string{"pr", "boundary", "extra"}, 10, "", `evolve pr boundary: unexpected operand "extra"`},
		{"pr refresh refuses an operand", []string{"pr", "refresh", "extra"}, 10, "", "usage: evolve pr refresh [--project-root P]"},
		{"ci classify needs a target", []string{"ci", "classify"}, 10, "", "evolve ci classify: want exactly one target, got 0"},
		{"ci classify refuses a malformed target", []string{"ci", "classify", "nonsense"}, 10, "", `ciwatch: target must be <run-id>, run:<id>, pr:<n>, sha:<hex> or a 7-40 hex sha: "nonsense"`},
		{"ci watch help is a zero exit", []string{"ci", "watch", "--help"}, 0, "usage: evolve ci watch (--sha S | --pr N | --tag T)", ""},
		{"ci watch needs exactly one target", []string{"ci", "watch"}, 10, "", "evolve ci watch: name exactly one of --sha, --pr or --tag, got 0"},
		{"ci telemetry needs a jobs file", []string{"ci", "telemetry"}, 10, "", "evolve ci telemetry: want --jobs <file> and no operand"},
		{"ci darwin-set reaches the injected darwin-set verb", []string{"ci", "darwin-set", "bogus"}, 10, "", "usage: evolve ci darwin-set <generate|check> [--project-root P]"},
		{"ci telemetry reads the named jobs file", []string{"ci", "telemetry", "--jobs", filepath.Join(t.TempDir(), "absent-jobs.json")}, 2, "", "evolve ci telemetry: open "},
		{"pr window resolves the project root through the injected helper", []string{"pr", "window", "5", "--project-root", filepath.Join(t.TempDir(), "absent-root")}, 2, "", "evolve pr window: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), c.args...)
			if code < 0 {
				t.Fatalf("evolve %v did not start: %v", c.args, err)
			}
			if code != c.wantCode {
				t.Errorf("evolve %v: exit %d, want %d\nstdout:\n%s\nstderr:\n%s", c.args, code, c.wantCode, acsassert.OutputTail(stdout, 8), acsassert.OutputTail(stderr, 12))
			}
			if !strings.Contains(stdout, c.wantStdout) {
				t.Errorf("evolve %v: stdout lacks %q:\n%s", c.args, c.wantStdout, acsassert.OutputTail(stdout, 8))
			}
			if !strings.Contains(stderr, c.wantStderr) {
				t.Errorf("evolve %v: stderr lacks %q:\n%s", c.args, c.wantStderr, acsassert.OutputTail(stderr, 12))
			}
		})
	}
}

func TestC2564_005_TheNewPackageIsEnrolledInTheApicoverGateAndNamesEveryExport(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	enforce, err := os.ReadFile(filepath.Join(moduleDir, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(enforce), "\n"), landcmdPackage) {
		t.Errorf("go/.apicover-enforce has no line %q", landcmdPackage)
	}
	pkgDir := filepath.Join(moduleDir, "internal", "cli", "landcmd")
	exported := exportedSymbols(t, pkgDir)
	for _, want := range []string{"Deps", "RunCI", "RunPR"} {
		if !slices.Contains(exported, want) {
			t.Errorf("landcmd does not export %q", want)
		}
	}
	named, err := os.ReadFile(filepath.Join(pkgDir, "apicover_named_test.go"))
	if err != nil {
		t.Fatalf("landcmd has no apicover_named_test.go: %v", err)
	}
	for _, symbol := range exported {
		if !strings.Contains(string(named), symbol) {
			t.Errorf("apicover_named_test.go does not name the exported symbol %q", symbol)
		}
	}
}

func exportedSymbols(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	for _, file := range parsedFiles(t, dir, isProduction) {
		ast.FileExports(file)
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					names = append(names, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, s.Name.Name)
					case *ast.ValueSpec:
						for _, id := range s.Names {
							names = append(names, id.Name)
						}
					}
				}
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func definedInMainProduction(t *testing.T) map[string]bool {
	t.Helper()
	defined := map[string]bool{}
	for _, file := range parsedFiles(t, mainDir(t), isProduction) {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					defined[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if s, ok := spec.(*ast.TypeSpec); ok {
						defined[s.Name.Name] = true
					}
				}
			}
		}
	}
	return defined
}

func flagParserShapes(t *testing.T) []string {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	rels, err := acsassert.WalkModule(moduleDir, func(rel string) bool {
		inScope := strings.HasPrefix(rel, "cmd/evolve/") || strings.HasPrefix(rel, "internal/cli/")
		return inScope && strings.HasSuffix(rel, ".go") && isProduction(rel)
	})
	if err != nil {
		t.Fatal(err)
	}
	var homes []string
	fset := token.NewFileSet()
	for _, rel := range rels {
		file, err := parser.ParseFile(fset, filepath.Join(moduleDir, filepath.FromSlash(rel)), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if st, ok := spec.Type.(*ast.StructType); ok && hasFields(st, "bools", "values", "lists") {
				homes = append(homes, rel+":"+spec.Name.Name)
			}
			return true
		})
	}
	return homes
}

func hasFields(st *ast.StructType, want ...string) bool {
	have := map[string]bool{}
	for _, f := range st.Fields.List {
		for _, id := range f.Names {
			have[id.Name] = true
		}
	}
	for _, name := range want {
		if !have[name] {
			return false
		}
	}
	return true
}

func TestC2564_006_TheMovedVerbsLeaveNoDeadCopyInMainAndTheFlagParserHasOneHome(t *testing.T) {
	defined := definedInMainProduction(t)
	moved := []string{
		"runPRWindow", "runPRBoundary", "runPRRefresh", "runCIWatch", "runCITelemetry",
		"mergePRs", "newPRMerger", "decidePRWindow", "decidePRBoundary", "parseCIClassifyArgs",
		"writeClassifyReport", "parsePRMergeArgs", "prMerger",
	}
	for _, name := range moved {
		if defined[name] {
			t.Errorf("package main still defines %s", name)
		}
	}
	if homes := flagParserShapes(t); len(homes) != 1 {
		t.Errorf("the flag parser (a struct of bools, values and lists) has %d homes %v, want exactly 1: landcmd and main share one parser and keep no copy", len(homes), homes)
	}
}

func TestC2564_007_EveryTestOfTheMovedFilesIsKeptInMainOrInTheNewPackage(t *testing.T) {
	want := strings.Fields(movedTestNames)
	have := map[string]bool{}
	for _, dir := range []string{mainDir(t), filepath.Join(acsassert.GoModuleDir(t), "internal", "cli", "landcmd")} {
		for _, file := range parsedFiles(t, dir, isTest) {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
					have[fn.Name.Name] = true
				}
			}
		}
	}
	var missing []string
	for _, name := range want {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d of %d tests of the moved pr and ci files exist in neither go/cmd/evolve nor go/internal/cli/landcmd: %v", len(missing), len(want), missing)
	}
}

func assertTestPackageCompiles(t *testing.T, pkg string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-tags", "integration", "-count=1", "-run", "^$", pkg)
	if err != nil && code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	if code != 0 {
		t.Errorf("go test -tags integration -run ^$ %s exit %d: a test file still uses a symbol that moved\n%s", pkg, code, acsassert.OutputTail(stdout+stderr, 20))
	}
}

func TestC2564_008_TheTestPackagesCompileAfterTheMove(t *testing.T) {
	assertTestPackageCompiles(t, "./cmd/evolve")
	assertTestPackageCompiles(t, "./internal/cli/landcmd")
}

func TestC2564_009_TheSignalCenterRootListNoLongerNamesTheMovedCiWatchHandler(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-run", "^TestRootSignalCenter_EveryRootSubscribesThePublisher$", "./cmd/evolve")
	if err != nil && code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	if code != 0 {
		t.Errorf("TestRootSignalCenter_EveryRootSubscribesThePublisher exit %d: cmd_ci_watch.go:runCIWatch left main, so the root list in cmd_root_guard_test.go must drop it\n%s", code, acsassert.OutputTail(stdout+stderr, 20))
	}
}

const movedTestNames = `
TestCIWatchVerb_ARedRunReachesTheErrorsChannelWithItsCode
TestCIWatchVerb_ATagWatchPublishesOneEventForEachReleaseWorkflow
TestCIWatchVerb_PublishesCICompletedForEachWatchedWorkflow
TestClassifyRedRuns_AnUnloadablePolicyClassifiesNothing
TestDecidePRWindow_AFileNameThatIsNotJSONIsAnError
TestDecidePRWindow_EachUnreadableInputIsAnError
TestDecidePRWindow_ReadsThePRFilesWithTheirOldNamesThroughGH
TestDefaultBoundaryPlane_ReadsThePoolOfTheLastRecordedWave
TestFailingWhat_PrefersJobsThenTestThenTheRunURL
TestFileBaseRedItems_NamesEachNewItemOnStderrAndReportsAnUnreadableInbox
TestGoModulePath_Cases
TestGoModulePath_ReadsTheModuleLine
TestNewPRMerger_AnUnreadableWindowStageFailsAndTheOpenGateIsRead
TestNewPRMerger_CarriesTheTrailerStageOfPolicy
TestNewPRMerger_RefusesAnInvalidPolicy
TestNewPRMerger_TakesMethodAndPollFromPolicyAndWaitFromArgs
TestOrDash
TestParseCIClassifyArgs_OneTargetAndKnownFlags
TestParseCIWatchArgs
TestParsePRMergeArgs_InterleavesAndRejectsUsage
TestParsePRWindowArgs_TakesOnePROrAFileList
TestPlaneLiveRuns_SeesSiblingWorktreesOnly
TestPRBoundary_AnUnreadableWorkingDirectoryIsAnIOError
TestPRBoundary_AWindowPRDoesNotWaitAndTheBoundComesFromPolicy
TestPRBoundary_FailsLoudOnWhatItCannotRead
TestPRBoundary_NoLiveLoopIsAlreadyABoundary
TestPRBoundary_TheLongestGreenWaitPastTheBoundWithAnUnderfilledPoolCallsAnEarlyBoundary
TestPRBoundary_UsageAndAnAbsentPlane
TestPRMerge_AcceptsADeclaredTrailerInEnforce
TestPRMerge_ALoopThatStartsDuringTheWaitRefusesABoundaryPRAtTheMerge
TestPRMerge_AnOpenWindowFailsLoudOnAnUnreadableWindowOrPlane
TestPRMerge_AnOpenWindowWithAnIdlePlaneMergesWithoutTheWindowRule
TestPRMerge_BelowTheOpenWindowALiveLoopRefusesEveryPRAsToday
TestPRMerge_ReadsTheBodyAndCommitsOfThePR
TestPRMerge_RefusesAnIdThatIsNotOpenInEnforce
TestPRMerge_RefusesAPRWithoutATrailerInEnforce
TestPRMerge_ShadowStageWarnsAndStillMerges
TestPRMerge_WithALiveLoopAndTheWindowOnADocsPRMergesAndAGoPRIsRefused
TestPRMergeJourney_UpdateBranchTrainWithAPendingNewHead
TestPRMerger_AFailedUpdateBranchStopsBeforeCI
TestPRMerger_GHFailuresExitTwo
TestPRMerger_HeadWaitBoundIsAtLeastTheMoveBound
TestPRMerger_HeadWaitOutlastsTheMoveBoundWhenWaitIsLonger
TestPRMerger_MergeOneFailurePaths
TestPRMerger_RefusesAndStops
TestPRMerger_TrainWithUpdateBranch
TestPRMerger_UpdateBranchRefreshesTheHeadBeforeTheMerge
TestPRMerger_UpdateBranchTimeout
TestPRMerger_UpdateBranchWithoutWaitMergesAGreenNewHead
TestPRMerger_UpdateBranchWithoutWaitRefusesAPendingNewHead
TestPRMerger_UpdateBranchWithoutWaitRefusesAtTheBound
TestPRMerger_WaitsForGreenThenMergesAtTheVerifiedHead
TestPRMerger_WaitsForNewHeadAfterUpdateBranch
TestPRRefresh_AConflictOutsideTheGeneratedRegionIsNeverStaged
TestPRRefresh_AFailedGeneratorAUsageErrorAndAGitFaultFailLoudly
TestPRRefresh_AnEntryTheCatalogLacksFailsLoudly
TestPRRefresh_EachFailedGitStepFailsTheRefresh
TestPRRefresh_InAConflictedMergeItRegeneratesTheIndexAndStagesIt
TestPRRefresh_RegeneratesTheThreeDerivedFilesAndCommitsNothingElse
TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex
TestPRWindow_AnEmptyChangeIsABoundary
TestPRWindow_AnUnreadableWorkingDirectoryExitsTwo
TestPRWindow_AtStageOnACodePathIsABoundaryAndExitsThree
TestPRWindow_AtStageOnADocsAndCIPRIsAWindowAndExitsZero
TestPRWindow_IOFailuresExitTwo
TestPRWindow_UntilIntegrationEnforceEveryPRIsABoundaryAndTheRuleStillShows
TestPRWindow_UsageErrorsExitTen
TestRunCI_AnUnreadableWorkingDirectoryIsAnIOErrorForBothVerbs
TestRunCI_RoutesWatchAndPrintsItsUsage
TestRunCI_UsageExitsTen
TestRunCI_UsageNamesTheTelemetryVerb
TestRunCIClassify_ABaseRedFilesOneFixFirstItemAndASecondRunFilesNone
TestRunCIClassify_ABaseRedThatGoesGreenOnTheRerunFilesNothing
TestRunCIClassify_AFailedAcsStepKeepsItsTestRowsAndTheGoJobGetsItsGateRow
TestRunCIClassify_AFailedCoverChangedGateIsOneRowThatNamesTheStepAndTheBoundedCommand
TestRunCIClassify_AFailedJobAPIReadKeepsTodaysRowsAndWarnsOnStderr
TestRunCIClassify_AGateRowTakesOnlyItsOwnJobOutOfARowThatTwoJobsShare
TestRunCIClassify_AGreenRunIsRetrySafe
TestRunCIClassify_AnUnwritableInboxIsAnIOErrorAfterTheReport
TestRunCIClassify_AnUnwritableStdoutIsAnIOError
TestRunCIClassify_ARedRunIsRefusedAndAMissingGoModWarns
TestRunCIClassify_ARunWithNoFailedGateStepKeepsTodaysOutputForEveryShape
TestRunCIClassify_EveryPrintedGateRowOfTheRecordedRunsIsPinnedWhole
TestRunCIClassify_RefusalsAndIOErrors
TestRunCIClassify_TheArtifactFormOfTheFaultGivesOneGateRowWithTodaysKey
TestRunCITelemetry_FailsLoudlyAndPrintsNoReport
TestRunCITelemetry_PrintsTheMarkdownReportAndTheJSONReport
TestRunCITelemetry_ReportsAWriteFailureInBothFormats
TestRunCIWatch_AFailedJobAPIReadAddsTheWarningOnStderrAndTheGateRowOtherwise
TestRunCIWatch_AFailedRunLookupExitsIOAndNamesTheWorkflow
TestRunCIWatch_AGreenSHAExits0
TestRunCIWatch_ARedPRFilesNoItemAndAFailedClassifyIsReported
TestRunCIWatch_ARedTagWatchesBothWorkflowsAndClassifiesTheRedRun
TestRunCIWatch_AWorkingDirectoryFaultExitsIOWhenNoRootIsGiven
TestRunCIWatch_InvalidTargetExitsUsage
TestRunCIWatch_NonexistentRootExitsIO
TestRunCIWatch_SetupAndFetchFailuresAreIOErrors
TestRunComments_IsCommentauditMain
TestRunPR_AnInvalidPolicyExitsTwo
TestRunPR_AnUnresolvableWorkingDirectoryExitsTwo
TestRunPR_ARootThatIsNoRepositoryCannotProveThePlaneIdle
TestRunPR_UsageExitsTen
TestWindowDecision_AListAtTheGitHubFileLimitIsABoundary
TestWriteClassifyReport_ADetailFollowsTheStepInTheThirdColumn
TestWriteClassifyReport_AMarshalFailureIsReturned
TestWriteClassifyReport_TextTableAndJSON
TestWriteClassifyWarnings_NamesEachWarningOnStderrWithThePrefix
`
