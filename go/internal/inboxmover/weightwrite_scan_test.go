package inboxmover

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

const (
	lifecycleWriterPrefix = "internal/inboxmover/lifecycle/"
)

func TestNoProductionCodeWritesAnItemWeightOutsideTheLifecycleWriter(t *testing.T) {
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range weightWriteSites(t, moduleDir) {
		t.Errorf("%s writes an inbox item weight: only %s writes one, through `evolve inbox edit --set weight=V --reason R`", site, lifecycleWriterPrefix)
	}
}

func TestWeightWriteSites_FindEveryShapeOfAWeightWriteAndSpareTheAllowedSites(t *testing.T) {
	module := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(module, "internal", "stray", "stray.go"), `package stray

import "example.test/inboxmover"

func Keyed() []inboxmover.FieldEdit {
	return []inboxmover.FieldEdit{{Op: inboxmover.EditSet, Field: "weight", Value: "0.9"}}
}

func Positional() inboxmover.FieldEdit {
	return inboxmover.FieldEdit{inboxmover.EditSet, "weight", "0.9"}
}

func Local() FieldEdit { return FieldEdit{Field: "weight"} }

func Indexed(item map[string]any) { item["weight"] = 0.9 }

func Priority(item map[string]any) { item["priority_class"] = "urgent" }
`)
	fixtures.MustWrite(t, filepath.Join(module, "internal", "stray", "creation.go"), `package stray

func Create() map[string]any {
	return map[string]any{"id": "new", "weight": 0.5}
}

func OtherField(item map[string]any) { item["title"] = "t" }
`)
	fixtures.MustWrite(t, filepath.Join(module, "internal", "stray", "stray_test.go"), `package stray

func Tested(item map[string]any) { item["weight"] = 0.9 }
`)
	fixtures.MustWrite(t, filepath.Join(module, "acs", "cycle1", "predicates_test.go"), `package cycle1

func Predicate(item map[string]any) { item["weight"] = 0.9 }
`)
	fixtures.MustWrite(t, filepath.Join(module, "internal", "inboxmover", "lifecycle", "edit.go"), `package lifecycle

func Edit(item map[string]any) { item["weight"] = 0.9 }
`)

	fixtures.MustWrite(t, filepath.Join(module, "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	fixtures.MustWrite(t, filepath.Join(module, "pkg", "lib", "lib.go"), "package lib\n")

	got := weightWriteSites(t, module)

	want := []string{
		"internal/stray/stray.go:6",
		"internal/stray/stray.go:10",
		"internal/stray/stray.go:13",
		"internal/stray/stray.go:15",
		"internal/stray/stray.go:17",
	}
	if !slices.Equal(got, want) {
		t.Errorf("weightWriteSites = %v, want %v", got, want)
	}
}

func weightWriteSites(t *testing.T, moduleDir string) []string {
	t.Helper()
	files, err := structure.SourceFiles(moduleDir, "cmd", "internal", "pkg")
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, rel := range files {
		if strings.HasPrefix(filepath.ToSlash(rel), lifecycleWriterPrefix) {
			continue
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, filepath.Join(moduleDir, rel), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", rel, parseErr)
		}
		for _, pos := range weightWritePositions(file) {
			sites = append(sites, fmt.Sprintf("%s:%d", filepath.ToSlash(rel), fset.Position(pos).Line))
		}
	}
	return sites
}

func weightWritePositions(file *ast.File) []token.Pos {
	var found []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			found = append(found, weightEditLiterals(v)...)
		case *ast.AssignStmt:
			for _, lhs := range v.Lhs {
				if idx, ok := lhs.(*ast.IndexExpr); ok && isReasonGuardedLiteral(idx.Index) {
					found = append(found, lhs.Pos())
				}
			}
		}
		return true
	})
	return found
}

func weightEditLiterals(lit *ast.CompositeLit) []token.Pos {
	if namesFieldEdit(lit.Type) {
		if setsWeightField(lit) {
			return []token.Pos{lit.Pos()}
		}
		return nil
	}
	list, ok := lit.Type.(*ast.ArrayType)
	if !ok || !namesFieldEdit(list.Elt) {
		return nil
	}
	var found []token.Pos
	for _, elt := range lit.Elts {
		if inner, ok := elt.(*ast.CompositeLit); ok && inner.Type == nil && setsWeightField(inner) {
			found = append(found, inner.Pos())
		}
	}
	return found
}

func namesFieldEdit(typ ast.Expr) bool {
	switch v := typ.(type) {
	case *ast.Ident:
		return v.Name == "FieldEdit"
	case *ast.SelectorExpr:
		return v.Sel.Name == "FieldEdit"
	}
	return false
}

func setsWeightField(lit *ast.CompositeLit) bool {
	for i, elt := range lit.Elts {
		kv, keyed := elt.(*ast.KeyValueExpr)
		switch {
		case keyed && isIdent(kv.Key, "Field") && isReasonGuardedLiteral(kv.Value):
			return true
		case !keyed && i == 1 && isReasonGuardedLiteral(elt):
			return true
		}
	}
	return false
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func isReasonGuardedLiteral(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	return err == nil && slices.Contains(inboxbatch.FieldsNeedingReason(), s)
}
