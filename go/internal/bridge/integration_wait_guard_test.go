package bridge

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

var launchWaitEntries = map[string]bool{
	"runTmuxREPL": true, "bootTmuxREPL": true, "prepareTmuxREPL": true, "execRunner": true,
	"launchPane": true, "launchSwept": true, "LaunchLLM": true, "BootSmokeTest": true,
	"Launch": true, "LaunchArgs": true,
}

func requiresIntegrationTag(f *ast.File) bool {
	for _, group := range f.Comments {
		if group.Pos() >= f.Package {
			break
		}
		for _, c := range group.List {
			expr, err := constraint.Parse(c.Text)
			if err == nil && constraint.IsGoBuild(c.Text) {
				return !expr.Eval(func(tag string) bool { return tag != "integration" })
			}
		}
	}
	return false
}

func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func isUnboundedContext(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Background" && sel.Sel.Name != "TODO" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "context"
}

func unboundedLaunchWaits(fset *token.FileSet, f *ast.File) []string {
	if !requiresIntegrationTag(f) {
		return nil
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := calleeName(call)
		if !launchWaitEntries[name] {
			return true
		}
		for _, arg := range call.Args {
			if isUnboundedContext(arg) {
				hits = append(hits, fmt.Sprintf("%s: %s", fset.Position(call.Pos()), name))
			}
		}
		return true
	})
	return hits
}

func TestIntegrationTests_EveryLaunchAndWaitHasABoundedContext(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var hits []string
	scanned := 0
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if requiresIntegrationTag(f) {
			scanned++
		}
		hits = append(hits, unboundedLaunchWaits(fset, f)...)
	}
	if scanned == 0 {
		t.Fatal("the guard found no integration-tagged test file, so it checks nothing; fix the scan")
	}
	if len(hits) > 0 {
		t.Fatalf("an integration test passes context.Background() or context.TODO() to a launch or wait entry, so a stuck agent waits until the package timeout panic (the #1132 macOS hang); give each call a bounded context:\n%s", strings.Join(hits, "\n"))
	}
}

func TestUnboundedLaunchWaits_FindsOnlyADirectUnboundedContextInAnIntegrationFile(t *testing.T) {
	t.Parallel()
	const body = `
package bridge

func f(ctx context.Context, e *Engine) {
	runTmuxREPL(context.Background(), nil, Deps{}, tmuxLaunch{})
	e.Launch(context.TODO(), req)
	runTmuxREPL(ctx, nil, Deps{}, tmuxLaunch{})
	itTmuxCtl.KillSession(context.Background(), "s")
	other.Background()
	pkg.x.Background()
	runTmuxREPL(nil)
	func() {}()
}
`
	cases := []struct {
		name, header string
		want         int
	}{
		{"integration", "//go:build integration\n", 2},
		{"integration and darwin", "//go:build integration && darwin\n", 2},
		{"not integration", "//go:build !integration\n", 0},
		{"untagged", "", 0},
		{"plain comment", "// integration\n", 0},
	}
	for _, tc := range cases {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x_test.go", tc.header+body, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if got := unboundedLaunchWaits(fset, f); len(got) != tc.want {
			t.Errorf("%s: got %d hits %v, want %d", tc.name, len(got), got, tc.want)
		}
	}
}

func TestRequiresIntegrationTag_ReadsOnlyTheHeader(t *testing.T) {
	t.Parallel()
	src := "package bridge\n\n//go:build integration\nfunc f() {}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "x_test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if requiresIntegrationTag(f) {
		t.Fatal("a build line after the package clause is no constraint")
	}
}
