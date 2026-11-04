package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var makeGoBuildLiterals = []string{"make", "-C", "go", "build"}

func stringLiteralArgs(call *ast.CallExpr) []string {
	var out []string
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			out = append(out, "")
			continue
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			out = append(out, "")
			continue
		}
		out = append(out, v)
	}
	return out
}

func hasConsecutive(args, want []string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		if reflect.DeepEqual(args[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

func makeGoBuildCallSites(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var sites []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && hasConsecutive(stringLiteralArgs(call), makeGoBuildLiterals) {
				sites = append(sites, fset.Position(call.Pos()).String())
			}
			return true
		})
	}
	sort.Strings(sites)
	return sites
}

func TestMakeGoBuild_TheCommandHasExactlyOneCallSite(t *testing.T) {
	t.Parallel()

	sites := makeGoBuildCallSites(t, ".")

	if len(sites) != 1 {
		t.Fatalf("`make -C go build` is spawned at %d non-test call sites %v; the boot refresh, the chain boundary and `evolve rebuild` must share one build function", len(sites), sites)
	}
}

func TestMakeGoBuildCallSiteScan_FindsASiteAndSkipsTestFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "package x\n\nfunc f() {\n\tcommand(ctx, \"make\", \"-C\", \"go\", \"build\")\n}\n\nfunc g() {\n\tcommand(ctx, \"make\", \"-C\", \"go\", target)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	sites := makeGoBuildCallSites(t, dir)

	if len(sites) != 1 || !strings.Contains(sites[0], "a.go:4") {
		t.Fatalf("sites = %v, want exactly the literal call in a.go line 4", sites)
	}
}

func TestRebuildVerbAndBootRefreshDefaultToOneBuildFunction(t *testing.T) {
	t.Parallel()

	verb := reflect.ValueOf(rebuildBuildFn).Pointer()
	boot := reflect.ValueOf(bootRefreshRebuildFn).Pointer()

	if verb != boot {
		t.Fatalf("rebuildBuildFn (%#x) and bootRefreshRebuildFn (%#x) are different functions; they must call one build function", verb, boot)
	}
}
