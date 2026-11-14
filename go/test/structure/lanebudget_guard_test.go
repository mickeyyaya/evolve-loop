package structure

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const laneBudgetImport = "github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"

var laneBudgetExempt = map[string]string{
	filepath.Join("internal", "releasepreflight", "releasepreflight.go"): "runs only on the console before a release, never in a lane",
}

func goTestArgvSites(root string, dirs []string, exempt map[string]string) ([]string, error) {
	files, err := SourceFiles(root, dirs...)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, rel := range files {
		if _, ok := exempt[rel]; ok {
			continue
		}
		file, fset, err := parseSource(root, rel)
		if err != nil {
			return nil, err
		}
		if lines := goTestArgvLines(fset, file, true); len(lines) > 0 {
			sites = append(sites, rel+":"+strings.Join(lines, ","))
		}
	}
	return sites, nil
}

func parseSource(root, rel string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
	return file, fset, err
}

type argvSite struct {
	line   int
	routed bool
}

func goTestArgvSitesIn(fset *token.FileSet, file *ast.File) []argvSite {
	pkg := importName(file, laneBudgetImport)
	runnerWrapsFile := pkg != "" && callsLaneBudget(file, pkg, "Runner")
	var sites []argvSite
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if isGoTestArgv(n) {
			sites = append(sites, argvSite{fset.Position(n.Pos()).Line, pkg != "" && (runnerWrapsFile || isRouted(stack, pkg))})
		}
		stack = append(stack, n)
		return true
	})
	return sites
}

func isGoTestArgv(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.CompositeLit:
		return len(node.Elts) > 0 && isStringLit(node.Elts[0], "test")
	case *ast.CallExpr:
		for i := 1; i < len(node.Args); i++ {
			if isStringLit(node.Args[i-1], "go") && isStringLit(node.Args[i], "test") {
				return true
			}
		}
	}
	return false
}

func isRouted(ancestors []ast.Node, pkg string) bool {
	for _, a := range ancestors {
		if call, ok := a.(*ast.CallExpr); ok && isLaneBudgetCall(call, pkg, "Args") {
			return true
		}
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if fn, ok := ancestors[i].(*ast.FuncDecl); ok {
			return callsLaneBudget(fn, pkg, "Args")
		}
	}
	return false
}

func isLaneBudgetCall(call *ast.CallExpr, pkg, member string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && isMemberOf(sel, pkg, []string{member})
}

func callsLaneBudget(root ast.Node, pkg, member string) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isLaneBudgetCall(call, pkg, member) {
			found = true
		}
		return !found
	})
	return found
}

func goTestArgvLines(fset *token.FileSet, file *ast.File, unroutedOnly bool) []string {
	var lines []string
	for _, site := range goTestArgvSitesIn(fset, file) {
		if !unroutedOnly || !site.routed {
			lines = append(lines, strconv.Itoa(site.line))
		}
	}
	return lines
}

func isStringLit(expr ast.Expr, want string) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == strconv.Quote(want)
}

func TestEveryGoTestArgvGoesThroughTheLaneBudget(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	sites, err := goTestArgvSites(root, []string{"internal", "cmd"}, laneBudgetExempt)
	if err != nil {
		t.Fatalf("goTestArgvSites = %v", err)
	}
	if len(sites) > 0 {
		t.Errorf("%d file(s) build a go test argv without internal/lanebudget; a lane must give each go test its CPU share (wave-orchestrator spec §12):\n%s", len(sites), strings.Join(sites, "\n"))
	}
	for rel := range laneBudgetExempt {
		file, fset, err := parseSource(root, rel)
		if err != nil || len(goTestArgvLines(fset, file, false)) == 0 {
			t.Errorf("exempt file %s = %v, want a file that still builds a go test argv; drop the stale exemption", rel, err)
		}
	}
}

func TestGoTestArgvSites_FindsEachArgvFormAndHonorsTheBudgetAndTheExemption(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/a/a.go":      "package a\nvar Args = []string{\"test\", \"./...\"}\n",
		"internal/b/b.go":      "package b\nimport \"os/exec\"\nfunc F() { _ = exec.Command(\"go\", \"test\", \"./x\") }\n",
		"internal/c/c.go":      "package c\nvar Args = append([]string{\"test\"}, \"./...\")\n",
		"internal/d/d.go":      "package d\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nvar Args = lanebudget.Args([]string{\"test\"})\n",
		"internal/e/e.go":      "package e\nvar Kinds = []string{\"chore\", \"test\"}\nvar M = map[string]string{\"test\": \"test\"}\nfunc F(a string) bool { return a == \"test\" }\n",
		"internal/f/f.go":      "package f\nvar Args = []string{\"test\"}\n",
		"internal/g/g_test.go": "package g\nvar Args = []string{\"test\"}\n",
		"internal/h/h.go":      "package h\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nvar Routed = lanebudget.Args([]string{\"test\"})\nvar Unrouted = []string{\"test\", \"./...\"}\n",
		"internal/i/i.go":      "package i\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nfunc F() { _ = lanebudget.Args(nil); _ = []string{\"test\", \"./...\"} }\n",
		"internal/k/k.go":      "package k\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nvar R = lanebudget.Runner(nil)\nfunc F() []string { return []string{\"test\", \"./...\"} }\n",
		"internal/j/j.go":      "package j\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nfunc F() { _ = lanebudget.Args(nil) }\nfunc G() { _ = []string{\"test\", \"./...\"} }\n",
	})

	sites, err := goTestArgvSites(root, []string{"internal"}, map[string]string{filepath.Join("internal", "f", "f.go"): "fixture"})

	want := []string{
		filepath.Join("internal", "a", "a.go") + ":2",
		filepath.Join("internal", "b", "b.go") + ":3",
		filepath.Join("internal", "c", "c.go") + ":2",
		filepath.Join("internal", "h", "h.go") + ":4",
		filepath.Join("internal", "j", "j.go") + ":4",
	}
	if err != nil || !slices.Equal(sites, want) {
		t.Errorf("goTestArgvSites = %q, %v, want %q", sites, err, want)
	}
}

func TestGoTestArgvSites_AnAbsentDirectoryOrABadFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"internal/bad.go": "package"})

	_, absent := goTestArgvSites(root, []string{"internal/gone"}, nil)
	_, bad := goTestArgvSites(root, []string{"internal"}, nil)

	if absent == nil || bad == nil {
		t.Errorf("goTestArgvSites = %v, %v, want an error for an absent directory and for a bad file", absent, bad)
	}
}

var laneBudgetMakeExempt = map[string]string{
	filepath.Join("cmd", "evolve", "cmd_wave.go"):              "runs make build on the console before a wave starts, never in a lane",
	filepath.Join("cmd", "evolve", "cmd_loop_boot_refresh.go"): "runs make build on the console at loop boot, never in a lane",
}

func makeSites(root string, dirs []string, exempt map[string]string) ([]string, error) {
	files, err := SourceFiles(root, dirs...)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, rel := range files {
		if _, ok := exempt[rel]; ok {
			continue
		}
		file, fset, err := parseSource(root, rel)
		if err != nil {
			return nil, err
		}
		if lines := makeLines(fset, file, true); len(lines) > 0 {
			sites = append(sites, rel+":"+strings.Join(lines, ","))
		}
	}
	return sites, nil
}

func makeLines(fset *token.FileSet, file *ast.File, unroutedOnly bool) []string {
	pkg := importName(file, laneBudgetImport)
	var lines []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		routed := pkg != "" && callsLaneBudget(fn, pkg, "Env")
		ast.Inspect(fn, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && runsMake(call) && (!unroutedOnly || !routed) {
				lines = append(lines, strconv.Itoa(fset.Position(call.Pos()).Line))
			}
			return true
		})
	}
	return lines
}

func runsMake(call *ast.CallExpr) bool {
	for i := 0; i < len(call.Args) && i < 2; i++ {
		if isStringLit(call.Args[i], "make") {
			return true
		}
	}
	return false
}

func TestEveryMakeRunGoesThroughTheLaneBudget(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	sites, err := makeSites(root, []string{"internal", "cmd"}, laneBudgetMakeExempt)
	if err != nil {
		t.Fatalf("makeSites = %v", err)
	}
	if len(sites) > 0 {
		t.Errorf("%d file(s) run make without lanebudget.Env in the same function; a make recipe runs go test and a lane must bound it:\n%s", len(sites), strings.Join(sites, "\n"))
	}
	for rel := range laneBudgetMakeExempt {
		file, fset, err := parseSource(root, rel)
		if err != nil || len(makeLines(fset, file, false)) == 0 {
			t.Errorf("exempt file %s = %v, want a file that still runs make; drop the stale exemption", rel, err)
		}
	}
}

func TestMakeSites_FindsAMakeRunWithoutTheLaneEnvInTheSameFunction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/a/a.go":      "package a\nimport \"os/exec\"\nfunc F(target string) { _ = exec.Command(\"make\", \"-C\", \"go\", target) }\n",
		"internal/b/b.go":      "package b\nfunc F(ctx int, target string) { _ = Command(ctx, \"make\", \"-C\", \"go\", target) }\nfunc Command(int, ...string) int { return 0 }\n",
		"internal/c/c.go":      "package c\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nfunc F(target string) { _ = lanebudget.Env(nil); _ = run(\"make\", target) }\nfunc run(...string) int { return 0 }\n",
		"internal/d/d.go":      "package d\nimport \"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget\"\nfunc F() { _ = lanebudget.Env(nil) }\nfunc G(target string) { _ = run(\"make\", target) }\nfunc run(...string) int { return 0 }\n",
		"internal/e/e.go":      "package e\nvar Kinds = []string{\"make\"}\nfunc F() string { return \"make\" }\n",
		"internal/f/f.go":      "package f\nimport \"os/exec\"\nfunc F() { _ = exec.Command(\"make\") }\n",
		"internal/g/g_test.go": "package g\nimport \"os/exec\"\nfunc F() { _ = exec.Command(\"make\") }\n",
		"internal/h/h.go":      "package h\nvar X = 1\n",
	})

	sites, err := makeSites(root, []string{"internal"}, map[string]string{filepath.Join("internal", "f", "f.go"): "fixture"})

	want := []string{
		filepath.Join("internal", "a", "a.go") + ":3",
		filepath.Join("internal", "b", "b.go") + ":2",
		filepath.Join("internal", "d", "d.go") + ":4",
	}
	if err != nil || !slices.Equal(sites, want) {
		t.Errorf("makeSites = %q, %v, want %q", sites, err, want)
	}
}

func TestMakeSites_AnAbsentDirectoryOrABadFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"internal/bad.go": "package"})

	_, absent := makeSites(root, []string{"internal/gone"}, nil)
	_, bad := makeSites(root, []string{"internal"}, nil)

	if absent == nil || bad == nil {
		t.Errorf("makeSites = %v, %v, want an error for an absent directory and for a bad file", absent, bad)
	}
}
