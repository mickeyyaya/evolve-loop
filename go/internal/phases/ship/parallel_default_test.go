package ship

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	seamRepoContract = "swaps the repoContractTestFn package variable through swapRepoContractTest"
	seamGoTestJSON   = "swaps the goTestJSONFn package variable through swapGoTestJSON"
	seamMarshalJSON  = "swaps the marshalJSON package variable through stubMarshalJSON"
	seamExecutable   = "swaps the osExecutable package variable through stubRunningExecutable"
	seamStdoutPipe   = "swaps the stdoutPipe package variable"
	seamInboxItem    = "swaps the readInboxItem and marshalInboxItem package variables"
)

var serialTests = map[string]string{
	"ship.TestAppendShipJournal_ReportsEveryWriteFailure":                       seamMarshalJSON,
	"ship.TestDiscardBinaryChurn_NeverRemovesRunningExecutable":                 seamExecutable,
	"ship.TestExplanationViewSHA_HashesTheSealedViewDeterministically":          seamMarshalJSON,
	"ship.TestItemConsumerStage_EveryFailureIsLoudAndLeavesTheItemPickable":     seamInboxItem,
	"ship.TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent":              seamExecutable,
	"ship.TestRepoContractGate_DiscoveryFailureIsInfra":                         seamRepoContract,
	"ship.TestRepoContractGate_EnforceGreenPasses":                              seamRepoContract,
	"ship.TestRepoContractGate_EnforceRedFailsWithDedicatedCode":                seamRepoContract,
	"ship.TestRepoContractGate_OffSkips":                                        seamRepoContract,
	"ship.TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns": seamRepoContract,
	"ship.TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry":        seamRepoContract,
	"ship.TestRepoContractGate_RedErrorMessageNamesFailingTests":                seamRepoContract,
	"ship.TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests":   seamRepoContract,
	"ship.TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns":               seamRepoContract,
	"ship.TestRepoContractGate_TransientFailureRetriesOnceThenShips":            seamRepoContract,
	"ship.TestRepoContractGate_UnknownStageFailsTowardEnforce":                  seamRepoContract,
	"ship.TestRepoContractPack_ShipAndTheBuildFloorMakeOneDecision":             seamRepoContract,
	"ship.TestRunGoTestJSON_AStdoutPipeFailureNeverStartsTheProcess":            seamStdoutPipe,
	"ship.TestRunNative_RepoContractGateReceivesRunWorkspace":                   seamRepoContract,
	"ship.TestRunRepoContractPack_NamesRedsOnlyForTheGatesRealRed":              seamRepoContract,
	"ship.TestRunRepoContractPack_RunsTheGatesOwnPackAndNamesItsReds":           seamRepoContract,
	"ship.TestRunRepoContractTestsAlone_EveryPackageGreenIsGreen":               seamGoTestJSON,
	"ship.TestRunRepoContractTestsAlone_KeepsTheUnprovenAndTheStillRed":         seamGoTestJSON,
}

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

func isStdDescriptorNewFile(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NewFile" || len(call.Args) == 0 {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "os" {
		return false
	}
	arg := call.Args[0]
	if conv, ok := arg.(*ast.CallExpr); ok && len(conv.Args) == 1 {
		arg = conv.Args[0]
	}
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	n, err := strconv.Atoi(lit.Value)
	return err == nil && n >= 0 && n <= 2
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
			if isStdDescriptorNewFile(v) {
				found = append(found, renderNode(fset, v))
			}
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
			violations = append(violations, key+": listed in serialTests but no such test exists")
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
		violations = append(violations, key+": calls t.Parallel but is listed in serialTests")
	case !parallelFirst && parallelAnywhere:
		violations = append(violations, key+" ("+file+"): t.Parallel must be the first statement of the test body")
	case !parallelFirst && exempt && listed:
		violations = append(violations, key+": calls t.Setenv or t.Chdir so it needs no serialTests entry")
	case !parallelFirst && !exempt && !listed:
		violations = append(violations, key+" ("+file+"): does not call t.Parallel and is not in serialTests")
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
	for _, v := range serialByDefaultViolations(readPackageSources(t), serialTests) {
		t.Error(v)
	}
}

func fixtureViolations(t *testing.T, serial map[string]string, extraSources map[string]string, testBody string) []string {
	t.Helper()
	sources := map[string]string{
		"fixture_test.go": "package ship\nimport (\n\t\"os\"\n\t\"testing\"\n)\nvar _ = os.Args\nvar sharedSeam = 1\nfunc TestFixture(t *testing.T) {\n" + testBody + "\n}\n",
	}
	for k, v := range extraSources {
		sources[k] = v
	}
	return serialByDefaultViolations(sources, serial)
}

func TestSerialByDefaultGuard_FixtureVerdicts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		body   string
		serial map[string]string
		want   string
	}{
		{"no t.Parallel is flagged", "\t_ = 1", nil, "ship.TestFixture (fixture_test.go): does not call t.Parallel and is not in serialTests"},
		{"t.Parallel first passes", "\tt.Parallel()", nil, ""},
		{"t.Parallel not first is flagged", "\t_ = 1\n\tt.Parallel()", nil, "t.Parallel must be the first statement"},
		{"t.Parallel inside a subtest only is flagged", "\tt.Run(\"a\", func(t *testing.T) { t.Parallel() })", nil, "t.Parallel must be the first statement"},
		{"renamed testing parameter is read", "\t_ = 1", nil, "does not call t.Parallel"},
		{"t.Setenv needs no entry", "\tt.Setenv(\"A\", \"b\")", nil, ""},
		{"t.Chdir needs no entry", "\tt.Chdir(\"/\")", nil, ""},
		{"setenv test with an entry is flagged", "\tt.Setenv(\"A\", \"b\")", map[string]string{"ship.TestFixture": "r"}, "needs no serialTests entry"},
		{"listed serial passes", "\t_ = 1", map[string]string{"ship.TestFixture": "r"}, ""},
		{"listed but parallel is flagged", "\tt.Parallel()", map[string]string{"ship.TestFixture": "r"}, "calls t.Parallel but is listed in serialTests"},
		{"listed but absent is flagged", "\tt.Parallel()", map[string]string{"ship.TestGone": "r"}, "ship.TestGone: listed in serialTests but no such test exists"},
		{"package variable write", "\tt.Parallel()\n\tsharedSeam = 2", nil, "must not change process state: sharedSeam = 2"},
		{"package variable write through the package name", "\tt.Parallel()\n\tship.sharedSeam = 2", nil, "ship.sharedSeam = 2"},
		{"package variable write inside a closure", "\tt.Parallel()\n\tfunc() { sharedSeam++ }()", nil, "sharedSeam++"},
		{"shadowing local is not a package write", "\tt.Parallel()\n\tsharedSeam := 1\n\tsharedSeam = 2", nil, ""},
		{"os.Stderr assignment", "\tt.Parallel()\n\tos.Stderr = nil", nil, "os.Stderr = nil"},
		{"os.Stdout assignment", "\tt.Parallel()\n\tos.Stdout = nil", nil, "os.Stdout = nil"},
		{"os.Stdin assignment", "\tt.Parallel()\n\tos.Stdin = nil", nil, "os.Stdin = nil"},
		{"os.Args assignment", "\tt.Parallel()\n\tos.Args = nil", nil, "os.Args = nil"},
		{"os.Chdir call", "\tt.Parallel()\n\t_ = os.Chdir(\"/\")", nil, "os.Chdir"},
		{"os.Setenv call", "\tt.Parallel()\n\t_ = os.Setenv(\"A\", \"b\")", nil, "os.Setenv"},
		{"os.Unsetenv call", "\tt.Parallel()\n\t_ = os.Unsetenv(\"A\")", nil, "os.Unsetenv"},
		{"os.Clearenv call", "\tt.Parallel()\n\tos.Clearenv()", nil, "os.Clearenv"},
		{"t.Setenv in a parallel test", "\tt.Parallel()\n\tt.Setenv(\"A\", \"b\")", nil, "t.Setenv"},
		{"t.Chdir in a parallel test", "\tt.Parallel()\n\tt.Chdir(\"/\")", nil, "t.Chdir"},
		{"os.NewFile on descriptor 0", "\tt.Parallel()\n\t_ = os.NewFile(0, os.DevNull)", nil, "os.NewFile(0, os.DevNull)"},
		{"os.NewFile on descriptor 2 through a conversion", "\tt.Parallel()\n\t_ = os.NewFile(uintptr(2), \"x\")", nil, "os.NewFile(uintptr(2)"},
		{"os.NewFile on a computed descriptor is allowed", "\tt.Parallel()\n\tfd := uintptr(9)\n\t_ = os.NewFile(fd, \"x\")", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := tc.body
			if tc.name == "renamed testing parameter is read" {
				got := serialByDefaultViolations(map[string]string{"r_test.go": "package ship\nimport \"testing\"\nfunc TestRenamed(tt *testing.T) {\n\ttt.Parallel()\n\t_ = 1\n}\nfunc TestRenamedBad(tt *testing.T) {\n\t_ = 1\n}\n"}, nil)
				if len(got) != 1 || !strings.Contains(got[0], "ship.TestRenamedBad") {
					t.Fatalf("a renamed parameter must be read: %v", got)
				}
				return
			}
			got := fixtureViolations(t, tc.serial, nil, body)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("want no violation, got %v", got)
				}
				return
			}
			if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), tc.want) {
				t.Fatalf("want a violation containing %q, got %v", tc.want, got)
			}
		})
	}
}

func TestSerialByDefaultGuard_PackageVariablesComeFromEveryFile(t *testing.T) {
	t.Parallel()
	production := map[string]string{"seam.go": "package ship\nvar productionSeam = 1\n"}
	got := fixtureViolations(t, nil, production, "\tt.Parallel()\n\tproductionSeam = 2")
	if len(got) != 1 || !strings.Contains(got[0], "productionSeam = 2") {
		t.Fatalf("a package variable declared in a non-test file must be seen: %v", got)
	}
}

func TestSerialByDefaultGuard_CatchesARealSeamTestMadeParallel(t *testing.T) {
	t.Parallel()
	sources := readPackageSources(t)
	const name = "TestRepoContractGate_OffSkips"
	for path, src := range sources {
		marker := "func " + name + "(t *testing.T) {\n"
		if strings.Contains(src, marker) {
			sources[path] = strings.Replace(src, marker, marker+"\tt.Parallel()\n", 1)
		}
	}
	got := serialByDefaultViolations(sources, serialTests)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "ship."+name+": calls t.Parallel but is listed in serialTests") {
		t.Fatalf("a seam-swapping test given t.Parallel must be flagged; got %v", got)
	}
}
