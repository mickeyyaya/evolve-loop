package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The check is AST-based, so comments and strings that mention os.Getenv do not trip it.
func TestRouter_NoEnvReads(t *testing.T) {
	srcs, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(srcs) == 0 {
		t.Fatal("no router source files found — wrong cwd?")
	}
	fset := token.NewFileSet()
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "os" && sel.Sel.Name == "Getenv" {
				t.Errorf("%s calls os.Getenv at %s — routing kernel must stay env-pure (config.Load is the sole env reader)",
					f, fset.Position(call.Pos()))
			}
			return true
		})
	}
}
