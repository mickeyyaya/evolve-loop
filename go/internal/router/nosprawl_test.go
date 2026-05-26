package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestRouter_NoEnvReads is the no-sprawl gate for the routing kernel. The
// router is a leaf decision package: every flag/config value must arrive via
// the injected config.RoutingConfig (read once in config.Load at the
// composition root), never by reading the process environment here. A stray
// os.Getenv call in this package would reintroduce the scattered-flag smell
// the dynamic-routing design exists to cure.
//
// Detection is AST-based (not substring) so comments and string literals
// mentioning os.Getenv don't trip the gate — only real call expressions do.
//
// Scope note: this gate covers the new routing kernel. The broader phase-flag
// cleanup (triage/tdd/buildplanner ShouldSkip) is the deferred task-5B
// follow-on and is intentionally out of scope here.
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
