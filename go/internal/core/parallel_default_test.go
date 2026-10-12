package core

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var processStateCalls = map[string]bool{
	"os.Chdir": true, "os.Setenv": true, "os.Unsetenv": true, "os.Clearenv": true,
}

var processStateVars = map[string]bool{
	"os.Stderr": true, "os.Stdout": true, "os.Stdin": true, "os.Args": true,
}

type parsedTestFile struct {
	path    string
	pkgName string
	file    *ast.File
}

func testingTName(fd *ast.FuncDecl) string {
	if fd.Type.Params == nil {
		return ""
	}
	for _, field := range fd.Type.Params.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "T" {
			continue
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "testing" {
			continue
		}
		if len(field.Names) == 1 && field.Names[0].Name != "_" {
			return field.Names[0].Name
		}
	}
	return ""
}

func isTopLevelTest(fd *ast.FuncDecl) bool {
	return fd.Recv == nil && fd.Body != nil &&
		strings.HasPrefix(fd.Name.Name, "Test") && fd.Name.Name != "TestMain"
}

func isMethodCallOn(n ast.Node, recv, method string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == recv
}

func startsWithParallel(body *ast.BlockStmt, tName string) bool {
	if tName == "" || len(body.List) == 0 {
		return false
	}
	first, ok := body.List[0].(*ast.ExprStmt)
	return ok && isMethodCallOn(first.X, tName, "Parallel")
}

func callsAnywhere(body *ast.BlockStmt, tName string, methods ...string) bool {
	if tName == "" {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		for _, method := range methods {
			if isMethodCallOn(n, tName, method) {
				found = true
			}
		}
		return !found
	})
	return found
}

func declaredPackageVars(files []parsedTestFile) map[string]bool {
	vars := map[string]bool{}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if name.Name != "_" {
						vars[name.Name] = true
					}
				}
			}
		}
	}
	return vars
}

func localNames(fd *ast.FuncDecl) map[string]bool {
	locals := map[string]bool{}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				locals[n.Name] = true
			}
		}
	}
	addFields(fd.Type.Params)
	addFields(fd.Type.Results)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				for _, l := range v.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						locals[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range v.Names {
				locals[id.Name] = true
			}
		case *ast.RangeStmt:
			if v.Tok == token.DEFINE {
				for _, e := range []ast.Expr{v.Key, v.Value} {
					if id, ok := e.(*ast.Ident); ok {
						locals[id.Name] = true
					}
				}
			}
		case *ast.FuncLit:
			addFields(v.Type.Params)
		}
		return true
	})
	return locals
}

func writtenPackageName(target ast.Expr, qualifier string, locals map[string]bool) string {
	for {
		switch v := target.(type) {
		case *ast.Ident:
			if locals[v.Name] {
				return ""
			}
			return v.Name
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && !locals[id.Name] {
				if processStateVars[id.Name+"."+v.Sel.Name] {
					return id.Name + "." + v.Sel.Name
				}
				if id.Name == qualifier {
					return v.Sel.Name
				}
			}
			target = v.X
		case *ast.IndexExpr:
			target = v.X
		case *ast.StarExpr:
			target = v.X
		case *ast.ParenExpr:
			target = v.X
		default:
			return ""
		}
	}
}

func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

func processStateMutations(fset *token.FileSet, fd *ast.FuncDecl, tName, qualifier string, pkgVars map[string]bool) []string {
	locals := localNames(fd)
	var found []string
	isShared := func(name string) bool { return processStateVars[name] || pkgVars[name] }
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				return true
			}
			for _, l := range v.Lhs {
				if isShared(writtenPackageName(l, qualifier, locals)) {
					found = append(found, renderNode(fset, v))
					break
				}
			}
		case *ast.IncDecStmt:
			if isShared(writtenPackageName(v.X, qualifier, locals)) {
				found = append(found, renderNode(fset, v))
			}
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					if processStateCalls[id.Name+"."+sel.Sel.Name] || (id.Name == tName && (sel.Sel.Name == "Setenv" || sel.Sel.Name == "Chdir")) {
						found = append(found, renderNode(fset, v))
					}
				}
			}
		}
		return true
	})
	return found
}

func serialByDefaultViolations(sources map[string]string, serial map[string]string) []string {
	fset := token.NewFileSet()
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var violations []string
	var files []parsedTestFile
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, sources[path], 0)
		if err != nil {
			violations = append(violations, path+": "+err.Error())
			continue
		}
		files = append(files, parsedTestFile{path: path, pkgName: file.Name.Name, file: file})
	}
	pkgVars := declaredPackageVars(files)
	seen := map[string]bool{}
	for _, pf := range files {
		if !strings.HasSuffix(pf.path, "_test.go") {
			continue
		}
		qualifier := strings.TrimSuffix(pf.pkgName, "_test")
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || !isTopLevelTest(fd) {
				continue
			}
			key := pf.pkgName + "." + fd.Name.Name
			seen[key] = true
			violations = append(violations, testViolations(fset, fd, key, filepath.Base(pf.path), qualifier, pkgVars, serial)...)
		}
	}
	for key := range serial {
		if !seen[key] {
			violations = append(violations, key+": listed in serialRequiredTests but no such test exists")
		}
	}
	sort.Strings(violations)
	return violations
}

func testViolations(fset *token.FileSet, fd *ast.FuncDecl, key, file, qualifier string, pkgVars map[string]bool, serial map[string]string) []string {
	tName := testingTName(fd)
	parallelFirst := startsWithParallel(fd.Body, tName)
	parallelAnywhere := callsAnywhere(fd.Body, tName, "Parallel")
	exempt := callsAnywhere(fd.Body, tName, "Setenv", "Chdir")
	_, listed := serial[key]
	var violations []string
	switch {
	case parallelFirst && listed:
		violations = append(violations, key+": calls t.Parallel but is listed in serialRequiredTests")
	case !parallelFirst && parallelAnywhere:
		violations = append(violations, key+" ("+file+"): t.Parallel must be the first statement of the test body")
	case !parallelFirst && exempt && listed:
		violations = append(violations, key+": calls t.Setenv or t.Chdir so it needs no serialRequiredTests entry")
	case !parallelFirst && !exempt && !listed:
		violations = append(violations, key+" ("+file+"): does not call t.Parallel and is not in serialRequiredTests")
	}
	if parallelAnywhere {
		for _, stmt := range processStateMutations(fset, fd, tName, qualifier, pkgVars) {
			violations = append(violations, key+" ("+file+"): a parallel test must not change process state: "+stmt)
		}
	}
	return violations
}

func readPackageSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob *.go: %v (%d files)", err, len(paths))
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sources[path] = string(raw)
	}
	return sources
}

func TestEveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	for _, v := range serialByDefaultViolations(readPackageSources(t), serialRequiredTests) {
		t.Error(v)
	}
}

func TestSerialByDefaultGuard_FlagsATestWithoutParallel(t *testing.T) {
	t.Parallel()
	source := "package core\nimport \"testing\"\nfunc TestFixtureSerial(t *testing.T) {\n\t_ = 1\n}\n"
	got := serialByDefaultViolations(map[string]string{"fixture_test.go": source}, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], "core.TestFixtureSerial") {
		t.Fatalf("a test without t.Parallel must be flagged exactly once; got %v", got)
	}
}

func TestSerialByDefaultGuard_AcceptsParallelListedAndEnvTests(t *testing.T) {
	t.Parallel()
	source := "package core\nimport \"testing\"\n" +
		"func TestFixtureParallel(t *testing.T) {\n\tt.Parallel()\n}\n" +
		"func TestFixtureListed(t *testing.T) {\n\t_ = 1\n}\n" +
		"func TestFixtureEnv(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}\n" +
		"func TestFixtureChdir(t *testing.T) {\n\tt.Chdir(\".\")\n}\n" +
		"func TestMain(m *testing.M) {}\n"
	listed := map[string]string{"core.TestFixtureListed": "fixture"}
	if got := serialByDefaultViolations(map[string]string{"fixture_test.go": source}, listed); len(got) != 0 {
		t.Fatalf("parallel, listed, Setenv and Chdir tests must pass; got %v", got)
	}
}

func TestSerialByDefaultGuard_FlagsStaleAndRedundantListEntries(t *testing.T) {
	t.Parallel()
	source := "package core\nimport \"testing\"\nfunc TestFixtureParallel(t *testing.T) {\n\tt.Parallel()\n}\n"
	listed := map[string]string{"core.TestFixtureParallel": "fixture", "core.TestGone": "fixture"}
	got := serialByDefaultViolations(map[string]string{"fixture_test.go": source}, listed)
	if len(got) != 2 {
		t.Fatalf("a listed test that calls t.Parallel and a listed test that is gone must both be flagged; got %v", got)
	}
}

func TestSerialByDefaultGuard_ParallelOnlyInSubtestDoesNotCount(t *testing.T) {
	t.Parallel()
	source := "package core\nimport \"testing\"\nfunc TestFixtureSub(t *testing.T) {\n\tt.Run(\"x\", func(t *testing.T) {\n\t\tt.Parallel()\n\t})\n}\n"
	if got := serialByDefaultViolations(map[string]string{"fixture_test.go": source}, nil); len(got) != 1 {
		t.Fatalf("t.Parallel only inside a subtest must not exempt the parent; got %v", got)
	}
}

func fixtureViolations(t *testing.T, pkgName, body string, serial map[string]string) []string {
	t.Helper()
	source := "package " + pkgName + "\nimport (\"os\"; \"testing\")\nvar sharedSeam = 1\nvar _ = os.Args\n" + body
	return serialByDefaultViolations(map[string]string{"fixture_test.go": source}, serial)
}

func TestSerialByDefaultGuard_ParallelMustBeTheFirstStatement(t *testing.T) {
	t.Parallel()
	late := "func TestFixtureLate(t *testing.T) {\n\t_ = 1\n\tt.Parallel()\n}\n"
	if got := fixtureViolations(t, "core", late, nil); len(got) != 1 || !strings.Contains(got[0], "first statement") {
		t.Fatalf("t.Parallel after another statement must be flagged as not first; got %v", got)
	}
	first := "func TestFixtureFirst(t *testing.T) {\n\tt.Parallel()\n\t_ = 1\n}\n"
	if got := fixtureViolations(t, "core", first, nil); len(got) != 0 {
		t.Fatalf("t.Parallel as the first statement must pass; got %v", got)
	}
}

func TestSerialByDefaultGuard_ReadsTheTestingTParameterName(t *testing.T) {
	t.Parallel()
	renamed := "func TestFixtureRenamed(tt *testing.T) {\n\ttt.Parallel()\n}\n"
	if got := fixtureViolations(t, "core", renamed, nil); len(got) != 0 {
		t.Fatalf("t.Parallel through a renamed parameter must pass; got %v", got)
	}
	wrongReceiver := "func TestFixtureWrong(tt *testing.T) {\n\tt := 1\n\t_ = t\n}\n"
	if got := fixtureViolations(t, "core", wrongReceiver, nil); len(got) != 1 {
		t.Fatalf("a test whose parameter is tt and that never calls tt.Parallel must be flagged; got %v", got)
	}
	blank := "func TestFixtureBlank(_ *testing.T) {\n\t_ = 1\n}\n"
	if got := fixtureViolations(t, "core", blank, nil); len(got) != 1 {
		t.Fatalf("a test with a blank testing.T parameter cannot be parallel and must be flagged; got %v", got)
	}
}

func TestSerialByDefaultGuard_ParallelTestMustNotMutateProcessState(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ pkg, stmt, want string }{
		"package variable":              {"core", "sharedSeam = 2", "sharedSeam = 2"},
		"package variable increment":    {"core", "sharedSeam++", "sharedSeam++"},
		"qualified package variable":    {"core_test", "core.Seam = 2", "core.Seam = 2"},
		"stderr":                        {"core", "os.Stderr = nil", "os.Stderr = nil"},
		"stdout":                        {"core", "os.Stdout = nil", "os.Stdout = nil"},
		"args":                          {"core", "os.Args = nil", "os.Args = nil"},
		"chdir":                         {"core", "_ = os.Chdir(\".\")", "os.Chdir"},
		"setenv":                        {"core", "_ = os.Setenv(\"A\", \"b\")", "os.Setenv"},
		"testing setenv":                {"core", "t.Setenv(\"A\", \"b\")", "t.Setenv"},
		"testing chdir":                 {"core", "t.Chdir(\".\")", "t.Chdir"},
		"mutation inside a closure":     {"core", "func() { sharedSeam = 3 }()", "sharedSeam = 3"},
		"mutation after a local define": {"core", "x := 1\n\t_ = x\n\tsharedSeam = 4", "sharedSeam = 4"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := "func TestFixtureMutates(t *testing.T) {\n\tt.Parallel()\n\t" + tc.stmt + "\n}\n"
			extra := ""
			if tc.pkg == "core_test" {
				extra = "var _ = 0\n"
			}
			source := "package " + tc.pkg + "\nimport (\"os\"; \"testing\")\nvar Seam = 1\nvar sharedSeam = 1\nvar _ = os.Args\n" + extra + body
			got := serialByDefaultViolations(map[string]string{"fixture_test.go": source}, nil)
			if len(got) != 1 || !strings.Contains(got[0], "TestFixtureMutates") || !strings.Contains(got[0], tc.want) {
				t.Fatalf("%s: want one violation naming the test and %q; got %v", name, tc.want, got)
			}
		})
	}
}

func TestSerialByDefaultGuard_ShadowedNamesAreNotPackageWrites(t *testing.T) {
	t.Parallel()
	body := "func TestFixtureShadow(t *testing.T) {\n\tt.Parallel()\n\tsharedSeam := 1\n\tsharedSeam = 2\n\t_ = sharedSeam\n}\n"
	if got := fixtureViolations(t, "core", body, nil); len(got) != 0 {
		t.Fatalf("a local variable that shadows a package variable is not a package write; got %v", got)
	}
}

func TestSerialByDefaultGuard_SerialListIsKeyedByPackageAndName(t *testing.T) {
	t.Parallel()
	internal := "package core\nimport \"testing\"\nfunc TestFixtureDup(t *testing.T) {\n\t_ = 1\n}\n"
	external := "package core_test\nimport \"testing\"\nfunc TestFixtureDup(t *testing.T) {\n\t_ = 1\n}\n"
	sources := map[string]string{"a_test.go": internal, "b_test.go": external}
	got := serialByDefaultViolations(sources, map[string]string{"core.TestFixtureDup": "fixture"})
	if len(got) != 1 || !strings.Contains(got[0], "core_test.TestFixtureDup") {
		t.Fatalf("listing core.TestFixtureDup must not exempt core_test.TestFixtureDup; got %v", got)
	}
}

func TestSerialByDefaultGuard_NonTestFilesSupplyPackageVariables(t *testing.T) {
	t.Parallel()
	production := "package core\nvar productionSeam = 1\n"
	test := "package core\nimport \"testing\"\nfunc TestFixtureProd(t *testing.T) {\n\tt.Parallel()\n\tproductionSeam = 2\n}\n"
	got := serialByDefaultViolations(map[string]string{"prod.go": production, "prod_test.go": test}, nil)
	if len(got) != 1 || !strings.Contains(got[0], "productionSeam = 2") {
		t.Fatalf("a package variable declared in a non-test file must be known; got %v", got)
	}
}

const (
	changesDirectory                = "test changes the process directory"
	setsEnv                         = "test sets the process environment through a helper"
	swapsBackoffSleep               = "test swaps package variable backoffSleep"
	swapsBuildSelfCheckRunner       = "test swaps package variable buildSelfCheckRunner"
	swapsChallengeTokenRead         = "test swaps package variable challengeTokenRead"
	swapsCoverChangedProfiler       = "test swaps package variable coverChangedProfiler"
	swapsCoverLockWait              = "test swaps package variable coverLockWait"
	swapsCoveringTestsDeriver       = "test swaps package variable coveringTestsDeriver"
	swapsFailureDecide              = "test swaps package variable failureDecide"
	swapsFailureDigestMarshal       = "test swaps package variable failureDigestMarshal"
	swapsGitRunner                  = "test swaps package variable gitRunner"
	swapsIdentityPatchID            = "test swaps package variable identityPatchID"
	swapsNextCorrection             = "test swaps package variable nextCorrection"
	swapsOsArgs                     = "test swaps os.Args"
	swapsPhaseBoundaryCheckpointer  = "test swaps package variable PhaseBoundaryCheckpointer"
	swapsPhasePlanMarshal           = "test swaps package variable phasePlanMarshal"
	swapsPostBuildRepinProvenanceFn = "test swaps package variable postBuildRepinProvenanceFn"
	swapsPostScoutReplanProbe       = "test swaps package variable postScoutReplanProbe"
	swapsPredicateModuleDir         = "test swaps package variable predicateModuleDir"
	swapsQuotaBoundaryCheckpointer  = "test swaps package variable QuotaBoundaryCheckpointer"
	swapsShipWindowAcquireTimeout   = "test swaps package variable shipWindowAcquireTimeout"
	swapsStderr                     = "test swaps os.Stderr"
	swapsWorktreeAddRetrySleep      = "test swaps package variable worktreeAddRetrySleep"
)

var serialRequiredTests = map[string]string{
	"core.TestDetectLostLanding_NoWorkspaceDoesNotReadTheProcessCWD":                                changesDirectory,
	"core.TestEnsureFailureDigest_MissingWorkspaceNeverWritesToWorkingDirectory":                    changesDirectory,
	"core.TestGitAndFileHelperCoverage":                                                             changesDirectory,
	"core.TestGitWorktree_RelativeBaseRefused":                                                      changesDirectory,
	"core.TestGitWorktree_RelativeProjectRootRefused":                                               changesDirectory,
	"core.TestNewCycleRun_RejectsRootCycleOccupiedInFetchedWorktree":                                setsEnv,
	"core.TestExecuteRetryBackoff_AppliedOnAttempt2":                                                swapsBackoffSleep,
	"core.TestExecuteRetryBackoff_ZeroBaseDisables":                                                 swapsBackoffSleep,
	"core.TestShipRecovery_JitteredBackoffBetweenReaudits":                                          swapsBackoffSleep,
	"core.TestBuildSelfCheck_ClearsStaleArtifactOnPass":                                             swapsBuildSelfCheckRunner,
	"core.TestBuildSelfCheck_NoGoChangesIsNoOp":                                                     swapsBuildSelfCheckRunner,
	"core.TestBuildSelfCheck_WritesArtifactOnFailure":                                               swapsBuildSelfCheckRunner,
	"core.TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags":                    swapsBuildSelfCheckRunner,
	"core.TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck":                                   swapsBuildSelfCheckRunner,
	"core.TestPersonaBudgetFailures":                                                                swapsBuildSelfCheckRunner,
	"core.TestRecordAndBranch_BuildGraduationGuardAborts":                                           swapsBuildSelfCheckRunner,
	"core.TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds":                                 swapsBuildSelfCheckRunner,
	"core.TestPlanCycle_ACallerTokenIsNeverReminted":                                                swapsChallengeTokenRead,
	"core.TestPlanCycle_AFailedTokenMintLeavesNoTokenAnywhere":                                      swapsChallengeTokenRead,
	"core.TestCoverChangedFailures_AFailingTestRefusesAtEnforceAndOnlyWarnsAtShadow":                swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_EnforceRefusesTheChangedFunctionOfTheBuild":                      swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_ShadowOffAndEveryPlumbingFaultLeaveTheHandoffAlone":              swapsCoverChangedProfiler,
	"core.TestScopedCoverFunc_BehindAHeldLockWaitsOnlyItsBoundThenRunsUnserialized":                 swapsCoverLockWait,
	"core.TestCoveringTests_AvailableToTestAmplificationWithoutFreshBuild":                          swapsCoveringTestsDeriver,
	"core.TestDecideAfterRetro_ABlockingDecisionEndsTheCycleNamingTheAction":                        swapsFailureDecide,
	"core.TestRecordAndBranch_ARetroBranchTheStateMachineForbidsAborts":                             swapsFailureDecide,
	"core.TestRecordAndBranch_ARetroDecisionCarriesItsEnvironmentIntoTheNextPhases":                 swapsFailureDecide,
	"core.TestResumeExecution_ARetroBranchTheStateMachineForbidsAbortsTheResume":                    swapsFailureDecide,
	"core.TestResumeExecution_ARetroDecisionCarriesItsEnvironmentIntoTheRescheduledPhase":           swapsFailureDecide,
	"core.TestAssembleFailureDigest_EachPersistenceFailureReturnsTheBuiltDigest":                    swapsFailureDigestMarshal,
	"core.TestCarryforwardCandidateLandable_CleanMergeIsLandable":                                   swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_ConflictIsFalseNotError":                                swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_PatchIdDuplicateSuperseded":                             swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_SupersededShortCircuitsBeforeMerge":                     swapsGitRunner,
	"core.TestClassifyFleetRebaseCandidate_GitInfraErrorIsError":                                    swapsGitRunner,
	"core.TestClassifyFleetRebaseCandidate_ThreeWayVerdicts":                                        swapsGitRunner,
	"core.TestCleanup_DeletesMergedCycleBranch":                                                     swapsGitRunner,
	"core.TestCleanup_UnmergedBranchSurvives_WarnsOnly":                                             swapsGitRunner,
	"core.TestDefaultGitHEAD_GitFailureDegrades":                                                    swapsGitRunner,
	"core.TestDefaultGitHEAD_UsesInjectedSeam":                                                      swapsGitRunner,
	"core.TestGitCapture_NonzeroExitReportedViaCode":                                                swapsGitRunner,
	"core.TestGitCapture_RoutesThroughInjectedSeam":                                                 swapsGitRunner,
	"core.TestGitWorktreeCleanup_NoWorktreeIsANoOp":                                                 swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_CleanRunCostsOneAttemptAndNoSleep":                              swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_PersistentFailureStillFailsLoudly":                              swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_RetriesTransientAddFailure":                                     swapsGitRunner,
	"core.TestGitWorktreeCreate_PersistentFailureStillFailsLoudly":                                  swapsGitRunner,
	"core.TestGitWorktreeCreate_RetriesTransientAddFailure":                                         swapsGitRunner,
	"core.TestGitWorktree_Create_FetchFailureIsFatal":                                               swapsGitRunner,
	"core.TestGitWorktree_Create_FetchesOriginBeforeBasingLane":                                     swapsGitRunner,
	"core.TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD":                                      swapsGitRunner,
	"core.TestGitWorktree_Create_ReuseSkipsFetch":                                                   swapsGitRunner,
	"core.TestLaneStartRef_FetchesOnlyTheDefaultBranch":                                             swapsGitRunner,
	"core.TestLaneStartRef_FollowsTheRemotesDefaultBranch":                                          swapsGitRunner,
	"core.TestLaneStartRef_PersistentContentionFailsAfterTheBound":                                  swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutAndBoundReportsOneReason":                            swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutMatchIsExactBranchName":                              swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutRefSkippedNotAborted":                                swapsGitRunner,
	"core.TestPruneSupersededOrphans_ContinuationBoundIsKept":                                       swapsGitRunner,
	"core.TestPruneSupersededOrphans_CorruptRegistryAbortsBeforeAnyDelete":                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_DeleteFailureReportedPerRefNotAborted":                         swapsGitRunner,
	"core.TestPruneSupersededOrphans_DistinctBranchUntouched":                                       swapsGitRunner,
	"core.TestPruneSupersededOrphans_HasOpenPRErrorAborts":                                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_HasOpenPRErrorStillAbortsAfterCheckedOutSkip":                  swapsGitRunner,
	"core.TestPruneSupersededOrphans_MultipleConsecutiveCheckedOutRefsAllSkipped":                   swapsGitRunner,
	"core.TestPruneSupersededOrphans_OpenPRFlaggedButKept":                                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_SupersededNoPRIsPruned":                                        swapsGitRunner,
	"core.TestPruneSupersededOrphans_WorktreeListFailureAbortsBeforeAnyDelete":                      swapsGitRunner,
	"core.TestRecordAuditBinding_AFailedHeadReadStopsBeforeAnyOtherGitOrBinding":                    swapsGitRunner,
	"core.TestRecoverBuildLeak_UsesInjectedGit":                                                     swapsGitRunner,
	"core.TestVerdictCacheCollisionRegression":                                                      swapsGitRunner,
	"core.TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim":                              swapsGitRunner,
	"core.TestIdentityCarryForward_AnUnidentifiablePatchIsReauditedAfterTheGates":                   swapsIdentityPatchID,
	"core.TestReviewWithCorrections_ALiveFixRungSpendsItsBudgetWithoutRedispatching":                swapsNextCorrection,
	"core_test.TestRecoverFromShipError_TheFenceKeepsTestNamesTheFenceErrorWhenKeptIsEmpty":         swapsOsArgs,
	"core_test.TestRunCycle_InterruptCheckpointsActivePhaseWithoutRetrospective":                    swapsPhaseBoundaryCheckpointer,
	"core.TestPhaseBoundaryCheckpointer_FiresDuringRunCycle":                                        swapsPhaseBoundaryCheckpointer,
	"core.TestRecordPhasePlanKind_AMarshalFailureLedgersThePlanWithoutAnArtifact":                   swapsPhasePlanMarshal,
	"core.TestBootRecovery_AfterBuildRepin_ShipGateSeesNoMismatch":                                  swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_PostBuildRepin_NoBinaryIsNoOp":                                           swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_PostBuildRepin_UnverifiedProvenance_KeepsPin":                            swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_RepinsAfterBuildNotJustBoot":                                             swapsPostBuildRepinProvenanceFn,
	"core.TestCycleLoop_PostScoutHookFiresOncePreBuild":                                             swapsPostScoutReplanProbe,
	"core.TestListACSPredicates_NoResolvableModuleIsALoudNote":                                      swapsPredicateModuleDir,
	"core.TestDispatch_AllFamiliesExhausted_NoFailureLearning":                                      swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycleFromPhase_AWalledResumedDispatchNamesItsWalk":                                 swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycleFromPhase_WalledResumedCorrection_Defers":                                     swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_ALedgerFaultDoesNotLoseTheQuotaPause":                                        swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AWalledPauseHandsTheWalkedCLIsToTheCheckpoint":                               swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamilies85_CheckpointsAndDefers":                                          swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro":                                   swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState":                             swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_FailedCorrectionRedispatch_IsNotADeferral":                                   swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_InterruptDuringQuotaCheckpointPreservesQuotaPause":                           swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_MixedExitCodes_NoDefer":                                                      swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_WalledCorrectionRedispatch_DefersLikeAFirstDispatch":                         swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_WalledRemediationRerun_DefersLikeAFirstDispatch":                             swapsQuotaBoundaryCheckpointer,
	"core.TestShipWindowWiring_FailOpenWhenSiblingHolds":                                            swapsShipWindowAcquireTimeout,
	"core.TestActiveVerifiedMints_ACorruptRegistryThatCannotBeQuarantinedWarnsTwice":                swapsStderr,
	"core.TestAdviseOnUnclassifiedFailure_RejectedAdviceIsNotPromoted":                              swapsStderr,
	"core.TestAdvisorStderrAndStream_HappyPathAreSilent":                                            swapsStderr,
	"core.TestApplyPostReviewGuards_AGitignoredBinaryIsNeverCheckedOut":                             swapsStderr,
	"core.TestApplyPostReviewGuards_AGuardThatCannotReSnapshotAfterTheDiscardAbortsUnchecked":       swapsStderr,
	"core.TestApplyPostReviewGuards_ALeakedPathUnderAConsoleLeaseContinuesButSaysItIsNotClean":      swapsStderr,
	"core.TestApplyPostReviewGuards_ARebuiltBinaryInsideTheMainTreeIsDiscardedAndThePhaseContinues": swapsStderr,
	"core.TestApplyPostReviewGuards_AnExecutableOutsideTheMainTreeOrUnknownLeavesNoRelativeBinary":  swapsStderr,
	"core.TestApplyPostReviewGuards_TheKnownGoEvolvePathIsNotDiscardedTwice":                        swapsStderr,
	"core.TestAuditorFindingsBrief_UnreadableReportWarns":                                           swapsStderr,
	"core.TestBridgeRetryAdjudicator_PromptCarriesTheAuditsReasonsFromTheOneReader":                 swapsStderr,
	"core.TestBuildTagVisiblePackages_DropsOnlyPackagesWithNoDefaultTagFiles":                       swapsStderr,
	"core.TestBuildTagVisiblePackages_GoListFailureFailsOpenWithAWarning":                           swapsStderr,
	"core.TestCommentFloorFailures_AnOffStageNeverReadsTheDiff":                                     swapsStderr,
	"core.TestCompleteCycle_AFailedToolOutputPruneWarnsAndStillSeals":                               swapsStderr,
	"core.TestComposeRepairBrief_ADefectTheBriefedFindingsCarryIsNotRepeated":                       swapsStderr,
	"core.TestComposeRepairBrief_AFailureBlockEveryDefectOfWhichIsBriefedAddsNoSection":             swapsStderr,
	"core.TestComposeRepairBrief_AnAgentGradedFailBriefsItsFailureBlockDefects":                     swapsStderr,
	"core.TestComposeRepairBrief_TheDedupeCoversExactlyTheBriefedFindings":                          swapsStderr,
	"core.TestComposeRepairBrief_TheRunnersReasonsNotTheWorkspaceFileDecideTheGateSlot":             swapsStderr,
	"core.TestCompositionCarryForward_DeclinesBeforeAnyGateOnEachUnverifiableComposition":           swapsStderr,
	"core.TestCompositionCarryForward_GateDeclineWritesNoUntaggedStderr":                            swapsStderr,
	"core.TestCoreFacades_ProjectTheLeaf":                                                           swapsStderr,
	"core.TestCoveringTests_AbsentCorpusIsAnnouncedNotSilent":                                       swapsStderr,
	"core.TestDecideAfterAuditFail_AnExplanationOnlyFailBriefsItsDefectsWithoutARecord":             swapsStderr,
	"core.TestDefaultBuildFloorChecks_WarnsOnSteFindingsInTheChangedDocs":                           swapsStderr,
	"core.TestDiscard_ANonRepositoryMainTreeIsNotRecovered":                                         swapsStderr,
	"core.TestDispatchSignals_DegradedDigestIsLoud":                                                 swapsStderr,
	"core.TestDispatch_ABestEffortObserverThatFailsAfterShipIsSkippedNotFatal":                      swapsStderr,
	"core.TestDispatch_ANonCanonicalVerdictAtTheRetryCapDegradesOnlyANonFloorPhase":                 swapsStderr,
	"core.TestDispatch_AReconciledTimeoutShipsOnTheAgentsVerdictAndLeavesAnAuditRow":                swapsStderr,
	"core.TestDispatch_ARetryRowThatCannotBeWrittenWarnsAndTheRelaunchStillHappens":                 swapsStderr,
	"core.TestDocsFloorWarn_ABrokenStandardWarnsThatTheLintDidNotRun":                               swapsStderr,
	"core.TestDocsFloorWarn_AChangedGeneratedDocIsNotAFinding":                                      swapsStderr,
	"core.TestDocsFloorWarn_AWorktreeWithoutTheStandardIsSilent":                                    swapsStderr,
	"core.TestDocsFloorWarn_AnUndocumentedArchitectureChangeWarnsByStage":                           swapsStderr,
	"core.TestDocsFloorWarn_CleanOrOutOfScopeDocsPrintNothing":                                      swapsStderr,
	"core.TestDocsFloorWarn_EveryOtherStageOnlyWarns":                                               swapsStderr,
	"core.TestDocsFloorWarn_NamesADocThatItCannotRead":                                              swapsStderr,
	"core.TestDocsFloorWarn_NamesTheFirstFindingOfTheChangedDocs":                                   swapsStderr,
	"core.TestDocsFloorWarn_SteStageOffPrintsNothing":                                               swapsStderr,
	"core.TestFloorStage_AnUnknownWordTurnsTheCheckOffAndNamesIt":                                   swapsStderr,
	"core.TestIdentityCarryForward_DeclinesWhenTheWorktreeCannotBeRead":                             swapsStderr,
	"core.TestIdentityCarryForward_GateDeclineEmitsOneCodedSignalEvent":                             swapsStderr,
	"core.TestLaneStartRef_RetriesRefLockContention":                                                swapsStderr,
	"core.TestMaterializedEvalSlugs_NoWorkspaceOrAnUnreadableHomeOwnsNothing":                       swapsStderr,
	"core.TestMaybeRemediate_AReRunThatTheReviewerRejectsAbortsTheCycle":                            swapsStderr,
	"core.TestMaybeRemediate_ARejectedBuilderFixVoidsTheRoundAndTheGateFailStands":                  swapsStderr,
	"core.TestMaybeRemediate_AResealedExplanationThatCannotBeProjectedVoidsTheRound":                swapsStderr,
	"core_test.TestNewCycleRun_DigestFailureWarnsNotAborts":                                         swapsStderr,
	"core.TestNormalizeScoutGoalHash_AnAbsentReportIsSilent":                                        swapsStderr,
	"core.TestNormalizeScoutGoalHash_AnUnreadableReportWarnsAndLeavesItAlone":                       swapsStderr,
	"core.TestNormalizeWorktreeToBase_ALockedBranchKeepsTheBuilderCommit":                           swapsStderr,
	"core.TestNormalizeWorktreeToBase_AnUnreadableHeadLeavesTheWorktreeAsIs":                        swapsStderr,
	"core.TestNoteContractExhaustionSkip_ALedgerFailureWarnsWithoutDroppingTheSkip":                 swapsStderr,
	"core.TestNoteContractGateDemotion_EachFailingSinkWarnsAndTheOthersStillRecord":                 swapsStderr,
	"core.TestPhaseAdvisor_SeesTheSignalCenterAppliedThroughTheOption":                              swapsStderr,
	"core.TestPlanCycle_AnUnwritableWorkspaceKeepsTheContextChallengeToken":                         swapsStderr,
	"core.TestPlanCycle_LedgerStampFailuresWarnAndThePlanStillCarriesTheDirectives":                 swapsStderr,
	"core.TestPlanCycle_ScopePathsSkipBlankIDsAndRefuseUnencodablePairs":                            swapsStderr,
	"core.TestPorcelainDirty_AnUnlaunchableGitIsAnErrorAndTheSetReadsClean":                         swapsStderr,
	"core.TestPostScoutReplan_AFailingAdvisorWarnsKeepsThePlanAndSpendsNoDepth":                     swapsStderr,
	"core.TestPostScoutReplan_AZeroDepthCapMeansOneReplan":                                          swapsStderr,
	"core.TestPostScoutReplan_AnUnreadableWorkspaceWarnsAndNeverReplans":                            swapsStderr,
	"core.TestReadContinuationFindings_RendersReasonsOrFallsBackLoudly":                             swapsStderr,
	"core.TestRecordAuditBinding_ALostLedgerRowStillCachesTheReusableVerdict":                       swapsStderr,
	"core.TestRecordAuditBinding_AnUnreadableProjectHeadBindsNothing":                               swapsStderr,
	"core.TestRecordAuditBinding_AnUnresolvableBaseSkipsTheCache":                                   swapsStderr,
	"core.TestRecordAuditBinding_AnUnwritableCacheWarnsAfterBinding":                                swapsStderr,
	"core.TestRecordBatchMember_ALedgerFailureWarnsAndTheBatchStillAdvances":                        swapsStderr,
	"core.TestRecordFailureLearning_DocMissingArmWritesTheDigestAndTheFallbackArmsNeverRewriteIt":   swapsStderr,
	"core.TestRecordFailureLearning_GateOrderIsNilCtxCarrierQuota":                                  swapsStderr,
	"core.TestRecordFailureLearning_NoRetroRunner_QueuesTodoAndPersistsWithoutRetroStamp":           swapsStderr,
	"core.TestRecordFailureLearning_PersistIsTheLastStorageEffect":                                  swapsStderr,
	"core.TestRecordFailureLearning_RetroLedgerAppendFailure_RaisesOnlyTheLedgerCode":               swapsStderr,
	"core.TestRecordFailureLearning_StreamIsByteIdenticalApartFromTheDeclaredCodes":                 swapsStderr,
	"core.TestRecordPhaseOutcome_AnAdvisoryOnAPassRidesTheRecord":                                   swapsStderr,
	"core.TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail":                   swapsStderr,
	"core.TestRecordPhasePlanKind_ALostAdvisorCaptureRowWarnsPerCapture":                            swapsStderr,
	"core.TestRecordProvisioningFailure_AnUncreatableWorkspaceWarnsAndWritesNoDigest":               swapsStderr,
	"core.TestRecordReplanEscalation_ALostLedgerRowWarnsAndNeverPanics":                             swapsStderr,
	"core.TestRecordRoutingDecision_AnUnencodableDecisionIsLedgeredWithoutAnArtifact":               swapsStderr,
	"core.TestRecoverFromShipError_AContinuationLaneRunsTheIdentityProof":                           swapsStderr,
	"core.TestRecoverFromShipError_AFailedPendAfterAnUnwindAborts":                                  swapsStderr,
	"core.TestRecoverFromShipError_ARouteTheStateMachineForbidsAborts":                              swapsStderr,
	"core.TestRecoverPath_AStagedAddOrDeleteIsDiscardedAndAnUnknownStatusAborts":                    swapsStderr,
	"core.TestRecoverTrackedEdit_AnEditThatVanishedFromTheMainTreeAborts":                           swapsStderr,
	"core.TestRecoverUntracked_AVanishedLeakCannotBeRelocated":                                      swapsStderr,
	"core.TestRemoveDuplicateLeak_AnUnremovableLeakIsNotRecovered":                                  swapsStderr,
	"core.TestRepairRoundDispatch_AnAgentGradedFailBriefsTheDefectsWithoutAWarn":                    swapsStderr,
	"core.TestRepoContractFloorChecks_ADeadlineKillWarnsAndPassesTheHandoff":                        swapsStderr,
	"core.TestRepoContractFloorChecks_GreenOrAnUnnamedExitPassesTheHandoff":                         swapsStderr,
	"core.TestRetireSupersededAuditArtifacts_ARenameAndRemoveRefusalLeavesTheSourceWithAWarning":    swapsStderr,
	"core.TestRetireSupersededAuditArtifacts_AStaleSourceThatCannotBeRemovedIsKeptAndWarned":        swapsStderr,
	"core.TestRetireSupersededAuditArtifacts_AnArchivePathPastPathMaxRemovesTheSourceUnarchived":    swapsStderr,
	"core.TestRetryPhaseRunner_ABackfilledArtifactTimeoutReturnsTheWarnResponse":                    swapsStderr,
	"core.TestReviewResumedDeliverable_ALeakWrittenByTheCorrectionStopsBeforeTheReReview":           swapsStderr,
	"core.TestReviewWithCorrections_ALeakAfterACorrectionAbortsBeforeTheReReview":                   swapsStderr,
	"core.TestReviewWithCorrections_ALostCorrectionLedgerRowWarnsAndTheLadderContinues":             swapsStderr,
	"core.TestRouteRebasedExplanation_ACommittedChangeReportsTheProofSkipped":                       swapsStderr,
	"core.TestRunCycle_AnUnwritableInteractionSummaryWarnsAndTheCycleStillCompletes":                swapsStderr,
	"core.TestRunCycle_PlanDegradeEmitsNoOrchestratorCode":                                          swapsStderr,
	"core.TestScopedMergeCarryForward_DeclinesOnEachFailClosedRung":                                 swapsStderr,
	"core.TestScopedMergeCarryForward_GateDeclineWritesNoUntaggedStderr":                            swapsStderr,
	"core.TestSetAsideShadowedLeak_AnUnwritableQuarantineKeepsTheLeakInPlace":                       swapsStderr,
	"core.TestStageRelocated_NothingToStageIsTrueAndAFailedAddIsFalse":                              swapsStderr,
	"core.TestStampContinuation_NeverSnapshotsTheProjectRoot":                                       swapsStderr,
	"core.TestWorktreeAddRetry_AnnouncementDoesNotClaimUnclassifiedTransience":                      swapsStderr,
	"core.TestWorktreeMutators_RefuseTheProjectRoot":                                                swapsStderr,
	"core.TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently":           swapsStderr,
	"core.TestWriteBuildSelfCheckArtifact_HealthyRoundTrip_LargeAndUnicodeContent":                  swapsStderr,
	"core.TestWriteBuildSelfCheckArtifact_MkdirAllFailure_WARNsAndDoesNotPanic":                     swapsStderr,
	"core.TestWriteBuildSelfCheck_HealthyWriteIsSilent":                                             swapsStderr,
	"core.TestWriteBuildSelfCheck_WriteFailureSurfaces":                                             swapsStderr,
	"core.TestWriteCoveringTests_SilentWhenNothingTruncated":                                        swapsStderr,
	"core.TestWriteCoveringTests_WarnsLoudlyOnTruncation":                                           swapsStderr,
	"core.TestLaneStartRef_RealLockOnTheBaseRefIsRetried":                                           swapsWorktreeAddRetrySleep,
}
