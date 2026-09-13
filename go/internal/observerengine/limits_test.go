package observerengine

// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 12 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	maxFuncLines = 50
	maxNesting   = 4
	maxFileLines = 800
)

func TestLimits_FunctionsFilesAndNestingStayWithinTheBar(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		if lines := strings.Count(string(src), "\n"); lines >= maxFileLines {
			t.Errorf("%s: %d lines ≥ %d", name, lines, maxFileLines)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		checkFunctions(t, fset, name, file)
	}
}

func checkFunctions(t *testing.T, fset *token.FileSet, name string, file *ast.File) {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lines := fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
		if lines >= maxFuncLines {
			t.Errorf("%s: %s is %d lines ≥ %d", name, fn.Name.Name, lines, maxFuncLines)
		}
		if depth := nesting(fn.Body, 0); depth > maxNesting {
			t.Errorf("%s: %s nests %d deep > %d", name, fn.Name.Name, depth, maxNesting)
		}
	}
}

// nesting returns the deepest block nesting inside a statement tree.
func nesting(node ast.Node, depth int) int {
	deepest := depth
	ast.Inspect(node, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if d := nesting(childBody(s), depth+1); d > deepest {
				deepest = d
			}
			return false
		}
		return true
	})
	return deepest
}

func childBody(s ast.Node) ast.Node {
	switch v := s.(type) {
	case *ast.IfStmt:
		return v.Body
	case *ast.ForStmt:
		return v.Body
	case *ast.RangeStmt:
		return v.Body
	case *ast.SwitchStmt:
		return v.Body
	case *ast.TypeSwitchStmt:
		return v.Body
	case *ast.SelectStmt:
		return v.Body
	}
	return s
}
