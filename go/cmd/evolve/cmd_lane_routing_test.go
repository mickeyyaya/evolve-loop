package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaneForbidden_TheRepoBuilderSandboxRoutesAProfileItemToTheConsole(t *testing.T) {
	var warn bytes.Buffer
	forbidden := laneForbidden(findRepoRoot(t), &warn)
	if !forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the builder sandbox denies .evolve/profiles, so no lane can create a profile")
	}
	if forbidden("go/internal/fleet/fleet.go") {
		t.Fatal("ordinary source stays lane work")
	}
	if warn.Len() != 0 {
		t.Fatalf("the repo's builder profile must load: %s", warn.String())
	}
}

func TestLaneForbidden_WithoutTheBuilderProfileWarnsAndJudgesProtectedSurface(t *testing.T) {
	var warn bytes.Buffer
	forbidden := laneForbidden(t.TempDir(), &warn)
	if !forbidden("go/internal/guards/role.go") || forbidden(".evolve/profiles/historian.json") {
		t.Fatal("the fallback judges protected surface only")
	}
	if strings.Count(warn.String(), "WARN") != 1 {
		t.Fatalf("a missing builder profile must be loud exactly once, got %q", warn.String())
	}
}

func TestLaneForbidden_ARootThatNeverRoutesStaysSilent(t *testing.T) {
	var warn bytes.Buffer
	_ = laneForbidden(t.TempDir(), &warn)
	if warn.Len() != 0 {
		t.Fatalf("building the predicate must not load or warn; got %q", warn.String())
	}
}

func TestRoutingRoots_JudgeWithTheLanePredicate(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	triageConfigSetsIt := false
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if id, isIdent := x.X.(*ast.Ident); isIdent && id.Name == "guards" && x.Sel.Name == "IsProtectedScope" && !(ok && fn.Name.Name == "laneForbidden") {
						t.Errorf("%s: a routing root passes guards.IsProtectedScope directly; use laneForbidden", fset.Position(x.Pos()))
					}
				case *ast.CompositeLit:
					if sel, isSel := x.Type.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Config" {
						if pkg, isIdent := sel.X.(*ast.Ident); isIdent && pkg.Name == "triage" {
							for _, elt := range x.Elts {
								if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
									if key, isIdent := kv.Key.(*ast.Ident); isIdent && key.Name == "LaneForbidden" {
										triageConfigSetsIt = true
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	if !triageConfigSetsIt {
		t.Error("the production triage.Config must set LaneForbidden")
	}
}
