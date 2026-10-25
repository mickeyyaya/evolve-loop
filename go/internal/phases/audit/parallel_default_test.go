package audit

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

var processGlobalSetters = map[string]bool{
	"SetModelCatalogDirFn": true, "RegisterCode": true, "UseLauncher": true, "SetHangClassifier": true,
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
			if id, ok := v.Fun.(*ast.Ident); ok && processGlobalSetters[id.Name] {
				found = append(found, renderNode(fset, v))
			}
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if processGlobalSetters[sel.Sel.Name] {
					found = append(found, renderNode(fset, v))
				}
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

func parallelDefaultViolations(sources map[string]string, serial map[string]string) []string {
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
	funcs := packageFuncs(files)
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
			violations = append(violations, testViolations(fset, fd, key, filepath.Base(pf.path), qualifier, pkgVars, funcs, serial)...)
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

func packageFuncs(files []parsedTestFile) map[string][]*ast.FuncDecl {
	funcs := map[string][]*ast.FuncDecl{}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				funcs[fd.Name.Name] = append(funcs[fd.Name.Name], fd)
			}
		}
	}
	return funcs
}

func reachableHelpers(root *ast.FuncDecl, funcs map[string][]*ast.FuncDecl) []*ast.FuncDecl {
	visited := map[*ast.FuncDecl]bool{root: true}
	queue := []*ast.FuncDecl{root}
	var reached []*ast.FuncDecl
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ast.Inspect(current.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			for _, callee := range funcs[id.Name] {
				if !visited[callee] {
					visited[callee] = true
					reached = append(reached, callee)
					queue = append(queue, callee)
				}
			}
			return true
		})
	}
	return reached
}

func testViolations(fset *token.FileSet, fd *ast.FuncDecl, key, file, qualifier string, pkgVars map[string]bool, funcs map[string][]*ast.FuncDecl, serial map[string]string) []string {
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
		for _, helper := range reachableHelpers(fd, funcs) {
			for _, stmt := range processStateMutations(fset, helper, testingTName(helper), qualifier, pkgVars) {
				violations = append(violations, key+" ("+file+"): a parallel test must not change process state via helper "+helper.Name.Name+": "+stmt)
			}
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

func TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	for _, v := range parallelDefaultViolations(readPackageSources(t), serialTests) {
		t.Error(v)
	}
}

func TestParallelDefaultGuard_FlagsATestWithoutParallel(t *testing.T) {
	t.Parallel()
	source := "package main\nimport \"testing\"\nfunc TestFixtureSerial(t *testing.T) {\n\t_ = 1\n}\n"
	got := parallelDefaultViolations(map[string]string{"fixture_test.go": source}, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], "main.TestFixtureSerial") {
		t.Fatalf("a test without t.Parallel must be flagged exactly once; got %v", got)
	}
}

func TestParallelDefaultGuard_AcceptsParallelListedAndEnvTests(t *testing.T) {
	t.Parallel()
	source := "package main\nimport \"testing\"\n" +
		"func TestFixtureParallel(t *testing.T) {\n\tt.Parallel()\n}\n" +
		"func TestFixtureListed(t *testing.T) {\n\t_ = 1\n}\n" +
		"func TestFixtureEnv(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}\n" +
		"func TestFixtureChdir(t *testing.T) {\n\tt.Chdir(\".\")\n}\n" +
		"func TestMain(m *testing.M) {}\n"
	listed := map[string]string{"main.TestFixtureListed": "fixture"}
	if got := parallelDefaultViolations(map[string]string{"fixture_test.go": source}, listed); len(got) != 0 {
		t.Fatalf("parallel, listed, Setenv and Chdir tests must pass; got %v", got)
	}
}

func TestParallelDefaultGuard_FlagsStaleAndRedundantListEntries(t *testing.T) {
	t.Parallel()
	source := "package main\nimport \"testing\"\nfunc TestFixtureParallel(t *testing.T) {\n\tt.Parallel()\n}\n"
	listed := map[string]string{"main.TestFixtureParallel": "fixture", "main.TestGone": "fixture"}
	got := parallelDefaultViolations(map[string]string{"fixture_test.go": source}, listed)
	if len(got) != 2 {
		t.Fatalf("a listed test that calls t.Parallel and a listed test that is gone must both be flagged; got %v", got)
	}
}

func TestParallelDefaultGuard_ParallelOnlyInSubtestDoesNotCount(t *testing.T) {
	t.Parallel()
	source := "package main\nimport \"testing\"\nfunc TestFixtureSub(t *testing.T) {\n\tt.Run(\"x\", func(t *testing.T) {\n\t\tt.Parallel()\n\t})\n}\n"
	if got := parallelDefaultViolations(map[string]string{"fixture_test.go": source}, nil); len(got) != 1 {
		t.Fatalf("t.Parallel only inside a subtest must not exempt the parent; got %v", got)
	}
}

func fixtureViolations(t *testing.T, pkgName, body string, serial map[string]string) []string {
	t.Helper()
	source := "package " + pkgName + "\nimport (\"os\"; \"testing\")\nvar sharedSeam = 1\nvar _ = os.Args\n" + body
	return parallelDefaultViolations(map[string]string{"fixture_test.go": source}, serial)
}

func TestParallelDefaultGuard_ParallelMustBeTheFirstStatement(t *testing.T) {
	t.Parallel()
	late := "func TestFixtureLate(t *testing.T) {\n\t_ = 1\n\tt.Parallel()\n}\n"
	if got := fixtureViolations(t, "main", late, nil); len(got) != 1 || !strings.Contains(got[0], "first statement") {
		t.Fatalf("t.Parallel after another statement must be flagged as not first; got %v", got)
	}
	first := "func TestFixtureFirst(t *testing.T) {\n\tt.Parallel()\n\t_ = 1\n}\n"
	if got := fixtureViolations(t, "main", first, nil); len(got) != 0 {
		t.Fatalf("t.Parallel as the first statement must pass; got %v", got)
	}
}

func TestParallelDefaultGuard_ReadsTheTestingTParameterName(t *testing.T) {
	t.Parallel()
	renamed := "func TestFixtureRenamed(tt *testing.T) {\n\ttt.Parallel()\n}\n"
	if got := fixtureViolations(t, "main", renamed, nil); len(got) != 0 {
		t.Fatalf("t.Parallel through a renamed parameter must pass; got %v", got)
	}
	wrongReceiver := "func TestFixtureWrong(tt *testing.T) {\n\tt := 1\n\t_ = t\n}\n"
	if got := fixtureViolations(t, "main", wrongReceiver, nil); len(got) != 1 {
		t.Fatalf("a test whose parameter is tt and that never calls tt.Parallel must be flagged; got %v", got)
	}
	blank := "func TestFixtureBlank(_ *testing.T) {\n\t_ = 1\n}\n"
	if got := fixtureViolations(t, "main", blank, nil); len(got) != 1 {
		t.Fatalf("a test with a blank testing.T parameter cannot be parallel and must be flagged; got %v", got)
	}
}

func TestParallelDefaultGuard_ParallelTestMustNotMutateProcessState(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ pkg, stmt, want string }{
		"package variable":              {"main", "sharedSeam = 2", "sharedSeam = 2"},
		"package variable increment":    {"main", "sharedSeam++", "sharedSeam++"},
		"qualified package variable":    {"main_test", "main.Seam = 2", "main.Seam = 2"},
		"stderr":                        {"main", "os.Stderr = nil", "os.Stderr = nil"},
		"stdout":                        {"main", "os.Stdout = nil", "os.Stdout = nil"},
		"args":                          {"main", "os.Args = nil", "os.Args = nil"},
		"chdir":                         {"main", "_ = os.Chdir(\".\")", "os.Chdir"},
		"setenv":                        {"main", "_ = os.Setenv(\"A\", \"b\")", "os.Setenv"},
		"testing setenv":                {"main", "t.Setenv(\"A\", \"b\")", "t.Setenv"},
		"testing chdir":                 {"main", "t.Chdir(\".\")", "t.Chdir"},
		"catalog dir setter":            {"main", "bridge.SetModelCatalogDirFn(nil)", "SetModelCatalogDirFn"},
		"register code setter":          {"main", "signalcenter.RegisterCode(nil)", "RegisterCode"},
		"launcher setter":               {"main", "modelquery.UseLauncher()", "UseLauncher"},
		"hang classifier setter":        {"main", "cycleclassify.SetHangClassifier(nil)", "SetHangClassifier"},
		"unqualified setter":            {"main", "SetModelCatalogDirFn(nil)", "SetModelCatalogDirFn"},
		"mutation inside a closure":     {"main", "func() { sharedSeam = 3 }()", "sharedSeam = 3"},
		"mutation after a local define": {"main", "x := 1\n\t_ = x\n\tsharedSeam = 4", "sharedSeam = 4"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := "func TestFixtureMutates(t *testing.T) {\n\tt.Parallel()\n\t" + tc.stmt + "\n}\n"
			extra := ""
			if tc.pkg == "main_test" {
				extra = "var _ = 0\n"
			}
			source := "package " + tc.pkg + "\nimport (\"os\"; \"testing\")\nvar Seam = 1\nvar sharedSeam = 1\nvar _ = os.Args\n" + extra + body
			got := parallelDefaultViolations(map[string]string{"fixture_test.go": source}, nil)
			if len(got) != 1 || !strings.Contains(got[0], "TestFixtureMutates") || !strings.Contains(got[0], tc.want) {
				t.Fatalf("%s: want one violation naming the test and %q; got %v", name, tc.want, got)
			}
		})
	}
}

func TestParallelDefaultGuard_ShadowedNamesAreNotPackageWrites(t *testing.T) {
	t.Parallel()
	body := "func TestFixtureShadow(t *testing.T) {\n\tt.Parallel()\n\tsharedSeam := 1\n\tsharedSeam = 2\n\t_ = sharedSeam\n}\n"
	if got := fixtureViolations(t, "main", body, nil); len(got) != 0 {
		t.Fatalf("a local variable that shadows a package variable is not a package write; got %v", got)
	}
}

func TestParallelDefaultGuard_SerialListIsKeyedByPackageAndName(t *testing.T) {
	t.Parallel()
	internal := "package main\nimport \"testing\"\nfunc TestFixtureDup(t *testing.T) {\n\t_ = 1\n}\n"
	external := "package main_test\nimport \"testing\"\nfunc TestFixtureDup(t *testing.T) {\n\t_ = 1\n}\n"
	sources := map[string]string{"a_test.go": internal, "b_test.go": external}
	got := parallelDefaultViolations(sources, map[string]string{"main.TestFixtureDup": "fixture"})
	if len(got) != 1 || !strings.Contains(got[0], "main_test.TestFixtureDup") {
		t.Fatalf("listing main.TestFixtureDup must not exempt main_test.TestFixtureDup; got %v", got)
	}
}

func TestParallelDefaultGuard_NonTestFilesSupplyPackageVariables(t *testing.T) {
	t.Parallel()
	production := "package main\nvar productionSeam = 1\n"
	test := "package main\nimport \"testing\"\nfunc TestFixtureProd(t *testing.T) {\n\tt.Parallel()\n\tproductionSeam = 2\n}\n"
	got := parallelDefaultViolations(map[string]string{"prod.go": production, "prod_test.go": test}, nil)
	if len(got) != 1 || !strings.Contains(got[0], "productionSeam = 2") {
		t.Fatalf("a package variable declared in a non-test file must be known; got %v", got)
	}
}

func TestParallelDefaultGuard_ParallelTestMustNotReachProcessStateThroughHelpers(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"setter two calls deep": "func helperOuter() { helperInner() }\nfunc helperInner() { bridge.SetModelCatalogDirFn(nil) }\n",
		"setenv in helper":      "func helperOuter() { _ = os.Setenv(\"A\", \"b\") }\n",
		"seam write in helper":  "func helperOuter() { sharedSeam = 5 }\n",
		"helper t.Setenv":       "func helperOuter(tb *testing.T) { tb.Setenv(\"A\", \"b\") }\n",
	}
	for name, helpers := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := "func TestFixtureHelper(t *testing.T) {\n\tt.Parallel()\n\thelperOuter()\n}\n"
			got := fixtureViolations(t, "main", helpers+body, nil)
			if len(got) != 1 || !strings.Contains(got[0], "TestFixtureHelper") || !strings.Contains(got[0], "via helper") {
				t.Fatalf("%s: want one violation naming the test and the helper; got %v", name, got)
			}
		})
	}
}

func TestParallelDefaultGuard_HelpersThatOnlyReadAreAccepted(t *testing.T) {
	t.Parallel()
	helpers := "func helperOuter() int { return sharedSeam }\n"
	body := "func TestFixtureHelperRead(t *testing.T) {\n\tt.Parallel()\n\t_ = helperOuter()\n}\n"
	if got := fixtureViolations(t, "main", helpers+body, nil); len(got) != 0 {
		t.Fatalf("a helper that only reads a package variable must pass; got %v", got)
	}
}
