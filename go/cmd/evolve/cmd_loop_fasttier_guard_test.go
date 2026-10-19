package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFastTierCoordinatorsNeverReachTheRealModuleGraphLoader(t *testing.T) {
	t.Parallel()
	reaching := map[string]bool{"dispatchFleetIteration": true, "dispatchPool": true, "takeSupply": true, "cellAdmission": true, "orchestratorPool": true, "moduleGraph": true}
	names, err := filepath.Glob("*_test.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("glob = %v, %v, want the test files of the package", names, err)
	}
	fset := token.NewFileSet()
	var offenders []string
	for _, name := range names {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if slowTier(src) {
			continue
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		callsReaching := false
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && reaching[sel.Sel.Name] {
				callsReaching = true
			}
			return true
		})
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			mustSet := callsReaching || returnsCoordinator(fn)
			ast.Inspect(fn, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if ok && mustSet && isCoordinatorLiteral(lit) && !setsKey(lit, "loadGraph") {
					offenders = append(offenders, fset.Position(lit.Pos()).String())
				}
				return true
			})
		}
	}
	if len(offenders) > 0 {
		t.Errorf("these loopBatchCoordinator literals can reach the module graph loader without a loadGraph, so a fast-tier test would run go list; set loadGraph: noModuleGraph:\n%s", strings.Join(offenders, "\n"))
	}
}

func slowTier(src []byte) bool {
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "package ") {
			return false
		}
		if strings.HasPrefix(line, "//go:build") && (strings.Contains(line, "integration") || strings.Contains(line, "e2e")) {
			return true
		}
	}
	return false
}

func returnsCoordinator(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if star, ok := field.Type.(*ast.StarExpr); ok {
			if id, ok := star.X.(*ast.Ident); ok && id.Name == "loopBatchCoordinator" {
				return true
			}
		}
	}
	return false
}

func isCoordinatorLiteral(lit *ast.CompositeLit) bool {
	id, ok := lit.Type.(*ast.Ident)
	return ok && id.Name == "loopBatchCoordinator"
}

func setsKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
				return true
			}
		}
	}
	return false
}
