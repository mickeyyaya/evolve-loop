package guards

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const sharedGoTestBudgetImport = "github.com/mickeyyaya/evolve-loop/go/internal/addedtests"

const sharedGoTestBudget = "PackageTimeout"

var goTestTimeoutFlags = []string{"-timeout", "-test.timeout"}

type goTestTimeout struct {
	at     token.Position
	shared bool
}

type budgetConst struct {
	value  ast.Expr
	budget string
}

func TestGoTestTimeouts_AreTheSharedPackageBudget(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sites, scanned := scanGoTestTimeouts(t, module)
	for _, want := range []string{"internal/core/phase_bindings_selfcheck.go", "cmd/evolve/main.go"} {
		if !slices.Contains(scanned, want) {
			t.Fatalf("the scan covers go/internal and go/cmd, so it must read %s; it read %d files", want, len(scanned))
		}
	}
	var seen, offenders []string
	for _, site := range sites {
		rel := moduleRelative(t, module, site.at.Filename)
		seen = append(seen, rel)
		if !site.shared {
			offenders = append(offenders, fmt.Sprintf("%s:%d", rel, site.at.Line))
		}
	}
	for _, want := range []string{"internal/core/phase_bindings_selfcheck.go", "internal/core/build_floor_reviewer.go", "internal/phases/ship/repocontract.go"} {
		if !slices.Contains(seen, want) {
			t.Fatalf("the scan must see the go-test -timeout in %s, or it is not reading the pipeline's argvs; it saw %v", want, seen)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("a go-test -timeout in pipeline code must be addedtests.PackageTimeout, the one budget the build floor, its tagged twin and ship's repo contract share; a deadline of its own is how the floor killed a green ./cmd/evolve at 120s in cycles 1787, 1791, 1792 and 1798:\n%s", strings.Join(offenders, "\n"))
	}
}

func TestFloorGoTestRunners_BuildTheirArgvFromTheBudgetedBuilders(t *testing.T) {
	for _, pin := range []struct{ path, runner, builder string }{
		{"../core/phase_bindings_selfcheck.go", "realGoUnitTest", "unitTestArgs"},
		{"../core/phase_bindings_selfcheck.go", "realGoUnitTestTagged", "taggedTestArgs"},
		{"../core/build_floor_reviewer.go", "scopedCoverFunc", "coverTestArgs"},
	} {
		if !functionCalls(t, pin.path, pin.runner, pin.builder) {
			t.Errorf("%s must build its go test argv with %s; an argv written in place can drop -timeout and run under Go's 10m default, which the -timeout scan cannot see", pin.runner, pin.builder)
		}
	}
}

func moduleRelative(t *testing.T, module, file string) string {
	t.Helper()
	rel, err := filepath.Rel(module, file)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func TestGoTestTimeouts_FlagEveryDeadlineButTheSharedBudget(t *testing.T) {
	cases := []struct {
		name, alias, expr string
		wantShared        []bool
	}{
		{"a literal after the flag in a call", "", `exec.Command("go", "test", "-timeout", "120s", pkg)`, []bool{false}},
		{"a literal after the flag in an argv", "", `[]string{"test", "-count=1", "-timeout", "300s"}`, []bool{false}},
		{"a literal appended to a go-test argv", "", `append([]string{"test", "-count=1"}, "-timeout", "5m")`, []bool{false}},
		{"a literal inline with the flag", "", `[]string{"test", "-timeout=20m"}`, []bool{false}},
		{"the test binary's own flag, which needs no test token", "", `exec.Command(bin, "-test.run", "TestX", "-test.timeout", "90s")`, []bool{false}},
		{"a local value", "", `[]string{"test", "-timeout", deadline}`, []bool{false}},
		{"the flag with its value elsewhere", "", `[]string{"test", "-timeout"}`, []bool{false}},
		{"a package const holding a literal", "", `[]string{"test", "-timeout", literalBudget}`, []bool{false}},
		{"another package's PackageTimeout", "", `[]string{"test", "-timeout", other.PackageTimeout}`, []bool{false}},
		{"the shared budget", "", `[]string{"test", "-timeout", addedtests.PackageTimeout}`, []bool{true}},
		{"the shared budget inline", "", `[]string{"test", "-timeout=" + addedtests.PackageTimeout}`, []bool{true}},
		{"a package const aliasing the shared budget", "", `[]string{"test", "-json", "-timeout", aliasBudget}`, []bool{true}},
		{"the shared budget under an import alias", "budget", `[]string{"test", "-timeout", budget.PackageTimeout}`, []bool{true}},
		{"evolve's own double-dash flag", "", `[]string{"loop-stop", "--wait", "--timeout", "5m"}`, nil},
		{"loop-stop's single-dash flag", "", `[]string{"loop-stop", "--wait", "-timeout", "5m"}`, nil},
		{"an argv reader", "", `slices.Contains(argv, "-timeout")`, nil},
		{"a prefix test on an argument", "", `strings.HasPrefix(a, "-timeout=")`, nil},
		{"another tool's -timeout", "", `exec.Command("curl", "-timeout", "5")`, nil},
		{"a go-test argv handed to a runner of its own", "", `o.runCmd(ctx, mod, "go", "test", "-timeout", "120s", pkg)`, []bool{false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf("package p\n\nimport %s %q\n\nconst aliasBudget = %s.PackageTimeout\n\nconst literalBudget = \"120s\"\n\nfunc f() { _ = %s }\n",
				tc.alias, sharedGoTestBudgetImport, cmp.Or(tc.alias, "addedtests"), tc.expr)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			var got []bool
			for _, site := range goTestTimeouts(fset, []*ast.File{file}) {
				got = append(got, site.shared)
			}
			if !slices.Equal(got, tc.wantShared) {
				t.Errorf("shared per -timeout in %s = %v, want %v", tc.expr, got, tc.wantShared)
			}
		})
	}
}

func scanGoTestTimeouts(t *testing.T, module string) (sites []goTestTimeout, scanned []string) {
	t.Helper()
	byDir := map[string][]string{}
	for _, tree := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(module, tree), func(p string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir() && d.Name() == "testdata":
				return filepath.SkipDir
			case !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go"):
				byDir[filepath.Dir(p)] = append(byDir[filepath.Dir(p)], p)
				scanned = append(scanned, moduleRelative(t, module, p))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(byDir)) {
		fset := token.NewFileSet()
		sites = append(sites, goTestTimeouts(fset, parseGoFiles(t, fset, byDir[dir]))...)
	}
	return sites, scanned
}

func parseGoFiles(t *testing.T, fset *token.FileSet, paths []string) []*ast.File {
	t.Helper()
	files := make([]*ast.File, 0, len(paths))
	for _, p := range paths {
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

func goTestTimeouts(fset *token.FileSet, files []*ast.File) []goTestTimeout {
	consts := packageBudgetConsts(files)
	var sites []goTestTimeout
	for _, file := range files {
		budget := budgetImportName(file)
		ast.Inspect(file, func(n ast.Node) bool {
			argv := argvOf(n)
			goTest := slices.ContainsFunc(argv, isTestToken)
			for i := range argv {
				if value, flag := timeoutValue(argv, i); flag == "-test.timeout" || (flag != "" && goTest) {
					sites = append(sites, goTestTimeout{at: fset.Position(argv[i].Pos()), shared: isSharedBudget(value, budget, consts)})
				}
			}
			return true
		})
	}
	return sites
}

func argvOf(n ast.Node) []ast.Expr {
	switch n := n.(type) {
	case *ast.CompositeLit:
		if isStringSlice(n.Type) {
			return n.Elts
		}
	case *ast.CallExpr:
		return n.Args
	}
	return nil
}

func isStringSlice(expr ast.Expr) bool {
	array, ok := expr.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	elem, ok := array.Elt.(*ast.Ident)
	return ok && elem.Name == "string"
}

func isTestToken(expr ast.Expr) bool {
	if text, ok := stringLiteralValue(expr); ok {
		return text == "test"
	}
	lit, ok := expr.(*ast.CompositeLit)
	return ok && isStringSlice(lit.Type) && slices.ContainsFunc(lit.Elts, isTestToken)
}

func timeoutValue(argv []ast.Expr, i int) (value ast.Expr, flag string) {
	if concat, ok := argv[i].(*ast.BinaryExpr); ok && concat.Op == token.ADD {
		if name, inline := inlineTimeoutFlag(concat.X); inline {
			return concat.Y, name
		}
	}
	if name, inline := inlineTimeoutFlag(argv[i]); inline {
		return argv[i], name
	}
	text, ok := stringLiteralValue(argv[i])
	if !ok || !slices.Contains(goTestTimeoutFlags, text) {
		return nil, ""
	}
	if i+1 < len(argv) {
		return argv[i+1], text
	}
	return nil, text
}

func inlineTimeoutFlag(expr ast.Expr) (flag string, inline bool) {
	text, ok := stringLiteralValue(expr)
	name, _, hasValue := strings.Cut(text, "=")
	if ok && hasValue && slices.Contains(goTestTimeoutFlags, name) {
		return name, true
	}
	return "", false
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(lit.Value)
	return text, err == nil
}

func isSharedBudget(value ast.Expr, budget string, consts map[string]budgetConst) bool {
	switch v := value.(type) {
	case *ast.SelectorExpr:
		pkg, ok := v.X.(*ast.Ident)
		return ok && budget != "" && pkg.Name == budget && v.Sel.Name == sharedGoTestBudget
	case *ast.Ident:
		def, ok := consts[v.Name]
		return ok && isSharedBudget(def.value, def.budget, consts)
	}
	return false
}

func budgetImportName(file *ast.File) string {
	for _, spec := range file.Imports {
		if importPath, err := strconv.Unquote(spec.Path.Value); err == nil && importPath == sharedGoTestBudgetImport {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return path.Base(importPath)
		}
	}
	return ""
}

func packageBudgetConsts(files []*ast.File) map[string]budgetConst {
	consts := map[string]budgetConst{}
	for _, file := range files {
		budget := budgetImportName(file)
		for _, spec := range constSpecs(file) {
			for i := range min(len(spec.Names), len(spec.Values)) {
				consts[spec.Names[i].Name] = budgetConst{value: spec.Values[i], budget: budget}
			}
		}
	}
	return consts
}

func constSpecs(file *ast.File) []*ast.ValueSpec {
	var specs []*ast.ValueSpec
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
			for _, spec := range gen.Specs {
				specs = append(specs, spec.(*ast.ValueSpec))
			}
		}
	}
	return specs
}
