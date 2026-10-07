package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const goModulePath = "github.com/mickeyyaya/evolve-loop/go/"

var chainBuilders = map[string][]string{
	goModulePath + "internal/llmroute":   {"Resolve", "ApplyUniversalFallback", "ChainFor", "ExcludeFamilies", "AllowedDiscovered"},
	goModulePath + "internal/resolvellm": {"Resolve"},
}

var chainBuilderHomes = []string{"internal/cliroute/", "internal/llmroute/", "internal/resolvellm/"}

func TestOnlyCliRouteBuildsChains(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	violations, scanned := chainBuilderCalls(t, module)
	if scanned < 500 {
		t.Fatalf("scanned only %d production files under %s — the guard lost its corpus", scanned, module)
	}
	for _, v := range violations {
		t.Errorf("%s builds a CLI chain outside internal/cliroute — resolve it through the Router (ADR-0119)", v)
	}
}

func TestChainBuilderCalls_FindsAnAliasedCallOutsideCliroute(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "x", "x.go"), `package x

import route "github.com/mickeyyaya/evolve-loop/go/internal/llmroute"

func f() { _ = route.Resolve }
`)
	writeGoFile(t, module, filepath.Join("internal", "cliroute", "ok.go"), `package cliroute

import "github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"

func g() { _, _ = resolvellm.Resolve("x", resolvellm.Options{}) }
`)
	writeGoFile(t, module, filepath.Join("internal", "x", "x_test.go"), `package x

import "github.com/mickeyyaya/evolve-loop/go/internal/llmroute"

func h() { _ = llmroute.ApplyUniversalFallback }
`)
	violations, scanned := chainBuilderCalls(t, module)
	if scanned != 2 || len(violations) != 1 || !strings.Contains(violations[0], filepath.Join("internal", "x", "x.go")+":5") {
		t.Fatalf("one aliased reference outside cliroute, tests and the home packages exempt: scanned=%d %v", scanned, violations)
	}
}

func chainBuilderCalls(t *testing.T, module string) ([]string, int) {
	t.Helper()
	var violations []string
	scanned := 0
	err := filepath.WalkDir(module, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(module, path)
		scanned++
		if slices.ContainsFunc(chainBuilderHomes, func(home string) bool { return strings.HasPrefix(filepath.ToSlash(rel), home) }) {
			return nil
		}
		violations = append(violations, chainBuilderRefs(t, path, rel)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return violations, scanned
}

func chainBuilderRefs(t *testing.T, path, rel string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	banned, dotImports := bannedSelectors(file)
	out := make([]string, 0, len(dotImports))
	for _, imp := range dotImports {
		out = append(out, rel+":"+strconv.Itoa(fset.Position(imp.Pos()).Line)+" dot-import of "+imp.Path.Value)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, isIdent := sel.X.(*ast.Ident)
		if !isIdent || !slices.Contains(banned[pkg.Name], sel.Sel.Name) {
			return true
		}
		out = append(out, rel+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+" "+pkg.Name+"."+sel.Sel.Name)
		return true
	})
	return out
}

func bannedSelectors(file *ast.File) (map[string][]string, []*ast.ImportSpec) {
	out := map[string][]string{}
	var dotImports []*ast.ImportSpec
	for _, imp := range file.Imports {
		importPath, _ := strconv.Unquote(imp.Path.Value)
		names, banned := chainBuilders[importPath]
		if !banned {
			continue
		}
		local := filepath.Base(importPath)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." {
			dotImports = append(dotImports, imp)
			continue
		}
		out[local] = names
	}
	return out, dotImports
}

func TestChainBuilderCalls_BansEveryChainPrimitiveOutsideCliroute(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "x", "x.go"), `package x

import "github.com/mickeyyaya/evolve-loop/go/internal/llmroute"

var (
	_ = llmroute.ChainFor
	_ = llmroute.ExcludeFamilies
	_ = llmroute.AllowedDiscovered
	_ = llmroute.Probe
)
`)
	violations, _ := chainBuilderCalls(t, module)
	if len(violations) != 3 {
		t.Fatalf("ChainFor, ExcludeFamilies and AllowedDiscovered build a chain; Probe only reorders one: %v", violations)
	}
}

func TestChainBuilderCalls_ADotImportOfAChainBuilderIsAViolation(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "x", "x.go"), `package x

import . "github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"

var _ = Resolve
`)
	violations, _ := chainBuilderCalls(t, module)
	if len(violations) != 1 || !strings.Contains(violations[0], "dot-import") {
		t.Fatalf("a dot-import hides every chain builder from the selector scan, so it is refused itself: %v", violations)
	}
}

func TestChainBuilderCalls_TheAdvisorsFileHasNoExceptionSinceL2(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "core", "advisor", "launch.go"), `package advisor

import "github.com/mickeyyaya/evolve-loop/go/internal/llmroute"

var (
	_ = llmroute.ChainFor
	_ = llmroute.ExcludeFamilies
)
`)
	violations, _ := chainBuilderCalls(t, module)
	if len(violations) != 2 || !strings.Contains(violations[0], "llmroute.ChainFor") {
		t.Fatalf("the advisor walks the routing table's chain since L2, so its ChainFor is a violation like any other: %v", violations)
	}
}
