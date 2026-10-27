package bridge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func isTypeLit(e ast.Expr, name string) bool {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return false
	}
	id, ok := lit.Type.(*ast.Ident)
	return ok && id.Name == name
}

func valueNamed(lhs []ast.Expr, rhs []ast.Expr, name string) ast.Expr {
	if len(lhs) != len(rhs) {
		return nil
	}
	for i, l := range lhs {
		if id, ok := l.(*ast.Ident); ok && id.Name == name {
			return rhs[i]
		}
	}
	return nil
}

func specValue(spec *ast.ValueSpec, name string) ast.Expr {
	lhs := make([]ast.Expr, len(spec.Names))
	for i, n := range spec.Names {
		lhs[i] = n
	}
	return valueNamed(lhs, spec.Values, name)
}

func packageRealTmuxVars(f *ast.File, vars map[string]bool) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				if isTypeLit(specValue(spec.(*ast.ValueSpec), name.Name), "execTmux") {
					vars[name.Name] = true
				}
			}
		}
	}
}

func isRealTmux(e ast.Expr, pkgVars map[string]bool) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return isTypeLit(e, "execTmux")
	}
	if id.Obj == nil {
		return pkgVars[id.Name]
	}
	switch decl := id.Obj.Decl.(type) {
	case *ast.AssignStmt:
		return isTypeLit(valueNamed(decl.Lhs, decl.Rhs, id.Name), "execTmux")
	case *ast.ValueSpec:
		return isTypeLit(specValue(decl, id.Name), "execTmux")
	}
	return false
}

func depsKeys(lit *ast.CompositeLit) map[string]ast.Expr {
	keys := map[string]ast.Expr{}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				keys[id.Name] = kv.Value
			}
		}
	}
	return keys
}

func realTmuxDepsWithoutSender(fset *token.FileSet, f *ast.File, names map[string]bool) (literals int, hits []string) {
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isTypeLit(lit, "Deps") {
			return true
		}
		keys := depsKeys(lit)
		if tmux, ok := keys["Tmux"]; !ok || !isRealTmux(tmux, names) {
			return true
		}
		literals++
		sender, ok := keys["SignalProcess"]
		if id, isIdent := sender.(*ast.Ident); !ok || isIdent && id.Name == "nil" {
			hits = append(hits, fmt.Sprint(fset.Position(lit.Pos())))
		}
		return true
	})
	return literals, hits
}

func scanRealTmuxDeps(fset *token.FileSet, files []*ast.File) (literals int, hits []string) {
	names := map[string]bool{}
	for _, f := range files {
		packageRealTmuxVars(f, names)
	}
	for _, f := range files {
		n, h := realTmuxDepsWithoutSender(fset, f, names)
		literals, hits = literals+n, append(hits, h...)
	}
	return literals, hits
}

func TestRealTmuxDeps_EveryTestDepsOverARealTmuxInjectsASender(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	literals, hits := scanRealTmuxDeps(fset, files)
	if literals == 0 {
		t.Fatal("the guard found no test Deps over execTmux{}, so it checks nothing; fix the scan")
	}
	if len(hits) > 0 {
		t.Fatalf("a test builds Deps over the real tmux without SignalProcess, so the reap at kill-session can send to a live pane through the default sender (PR #1190 macOS panic); set SignalProcess: refuseTestSignal:\n%s", strings.Join(hits, "\n"))
	}
}

func TestScanRealTmuxDeps_FindsEachRealTmuxLiteralWithNoSender(t *testing.T) {
	t.Parallel()
	const shared = `
package bridge

var ctl = execTmux{}
var a, b = 1, 2
var c, d = execTmux{}
`
	const body = `
package bridge

func f() {
	_ = Deps{Tmux: execTmux{}}
	_ = Deps{Tmux: execTmux{}, SignalProcess: nil}
	_ = Deps{Tmux: execTmux{}, SignalProcess: refuseTestSignal}
	tx := execTmux{}
	_ = Deps{Tmux: tx}
	_ = Deps{Tmux: ctl}
	fake, n := &fakeTmux{}, 1
	_ = Deps{Tmux: fake}
	_ = Deps{Tmux: &fakeTmux{}}
	_ = Deps{Tmux: c}
	_ = Deps{Tmux: d}
	x.y = execTmux{}
	_ = Deps{Sleep: nil}
	_ = Deps{execTmux{}}
	_ = other.Deps{Tmux: execTmux{}}
	_ = Other{Tmux: execTmux{}}
	_, _ = fake, n
}

func g(tx TmuxController) {
	_ = Deps{Tmux: tx}
	_ = Deps{Tmux: unknown}
	var local = execTmux{}
	_ = Deps{Tmux: local}
}
`
	fset := token.NewFileSet()
	var files []*ast.File
	for i, src := range []string{shared, body} {
		f, err := parser.ParseFile(fset, fmt.Sprintf("x%d_test.go", i), src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	literals, hits := scanRealTmuxDeps(fset, files)
	want := []string{"x1_test.go:5:", "x1_test.go:6:", "x1_test.go:9:", "x1_test.go:10:", "x1_test.go:28:"}
	if literals != 6 || len(hits) != len(want) {
		t.Fatalf("literals = %d, hits = %v; want 6 real tmux literals and hits on lines 5, 6, 9, 10 and 28", literals, hits)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(hits[i], prefix) {
			t.Errorf("hit %d = %s, want %s", i, hits[i], prefix)
		}
	}
}
