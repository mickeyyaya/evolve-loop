package bridge

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var ownServerStarters = map[string]bool{
	"startOwnTmuxServer":   true,
	"tmuxtest.StartServer": true,
	"startContractServer":  true,
}

func TestIntegrationTests_EveryOwnSocketStartsItsOwnServer(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, path := range paths {
		offenders = append(offenders, ownSocketsWithoutServer(t, path)...)
	}
	if len(offenders) > 0 {
		t.Errorf("%d integration test function(s) select their own tmux socket but start no server with exit-empty off; call ownTmuxServer:\n%s",
			len(offenders), strings.Join(offenders, "\n"))
	}
}

func TestOwnSocketsWithoutServer_FindsOnlyASocketWithNoStarter(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x_test.go")
	src := "//go:build integration\n\npackage x\n\n" +
		"func TestBare(t *testing.T) { t.Setenv(TmuxSocketEnv, \"s-bare\") }\n" +
		"func TestQualified(t *testing.T) { t.Setenv(bridge.TmuxSocketEnv, socket) }\n" +
		"func TestStarted(t *testing.T) { t.Setenv(TmuxSocketEnv, socket); startOwnTmuxServer(t, socket, nil) }\n" +
		"func TestShared(t *testing.T) { t.Setenv(bridge.TmuxSocketEnv, socket); _ = tmuxtest.StartServer(socket) }\n" +
		"func TestCleared(t *testing.T) { t.Setenv(TmuxSocketEnv, \"\") }\n" +
		"func TestOther(t *testing.T) { t.Setenv(\"HOME\", \"/x\") }\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(t.TempDir(), "y_test.go")
	if err := os.WriteFile(unit, []byte("package x\n\nfunc TestUnit(t *testing.T) { t.Setenv(TmuxSocketEnv, \"s\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := append(ownSocketsWithoutServer(t, path), ownSocketsWithoutServer(t, unit)...)

	if strings.Join(got, ",") != "x_test.go:TestBare,x_test.go:TestQualified" {
		t.Errorf("found %q, want only TestBare and TestQualified of the integration file", got)
	}
}

func ownSocketsWithoutServer(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if !needsIntegrationTag(file) {
		return nil
	}
	var offenders []string
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Body != nil && selectsOwnSocket(fd.Body) && !startsOwnServer(fd.Body) {
			offenders = append(offenders, filepath.Base(path)+":"+fd.Name.Name)
		}
	}
	return offenders
}

func needsIntegrationTag(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			return false
		}
		for _, c := range group.List {
			if expr, err := constraint.Parse(c.Text); err == nil {
				return !expr.Eval(func(tag string) bool { return tag != "integration" })
			}
		}
	}
	return false
}

func selectsOwnSocket(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || !strings.HasSuffix(exprName(call.Fun), ".Setenv") {
			return true
		}
		if name := exprName(call.Args[0]); name == "TmuxSocketEnv" || name == "bridge.TmuxSocketEnv" {
			lit, isLit := call.Args[1].(*ast.BasicLit)
			found = found || !isLit || lit.Value != strconv.Quote("")
		}
		return true
	})
	return found
}

func startsOwnServer(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && ownServerStarters[exprName(call.Fun)] {
			found = true
		}
		return !found
	})
	return found
}

func exprName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x := exprName(v.X); x != "" {
			return x + "." + v.Sel.Name
		}
	}
	return ""
}
