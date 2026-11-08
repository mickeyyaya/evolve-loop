package tmuxtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const moduleRoot = "../.."

type testPackageUse struct {
	startsTmux   bool
	callsTheMain bool
}

func TestEveryTmuxTestPackageUsesTheOrphanGuard(t *testing.T) {
	uses := scanTestPackages(t, moduleRoot)

	var unguarded []string
	for dir, use := range uses {
		if use.startsTmux && !use.callsTheMain && dir != "internal/tmuxtest" {
			unguarded = append(unguarded, dir)
		}
	}
	sort.Strings(unguarded)

	if len(unguarded) > 0 {
		t.Fatalf("these packages start a real tmux server in their tests without a TestMain that calls tmuxtest.Main, so the server can inherit and hold the go test output pipe: %v", unguarded)
	}
}

func TestStartsTmux_MatchesARealStartAndIgnoresAFixtureString(t *testing.T) {
	cases := map[string]bool{
		`exec.Command("tmux", "new-session")`:          true,
		`sysexec.Command(ctx, "tmux", "kill-server")`:  true,
		`exec.CommandContext(ctx, "tmux", "ls")`:       true,
		`testhost.Tool(t, "tmux")`:                     true,
		`exec.LookPath("tmux")`:                        true,
		`tmuxtest.StartServer(socket)`:                 true,
		`fixture("tmux", "sleep")`:                     false,
		`fmt.Sprint("sysexec.Command(ctx, \"tmux\")")`: false,
		`exec.Command("sleep", "60")`:                  false,
	}
	for expr, want := range cases {
		node, err := parser.ParseExpr(expr)
		if err != nil {
			t.Fatalf("parse %s: %v", expr, err)
		}
		if got := startsTmux(node); got != want {
			t.Errorf("startsTmux(%s) = %v, want %v", expr, got, want)
		}
	}
}

func scanTestPackages(t *testing.T, root string) map[string]testPackageUse {
	t.Helper()
	uses := map[string]testPackageUse{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		use := uses[filepath.ToSlash(rel)]
		use.startsTmux = use.startsTmux || startsTmux(file)
		use.callsTheMain = use.callsTheMain || callsSelector(file, "tmuxtest", "Main")
		uses[filepath.ToSlash(rel)] = use
		return nil
	})
	if err != nil {
		t.Fatalf("scan the test files under %s: %v", root, err)
	}
	return uses
}

func startsTmux(root ast.Node) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		found = isTmuxStart(call)
		return !found
	})
	return found
}

func isTmuxStart(call *ast.CallExpr) bool {
	pkg, name, ok := selectorName(call.Fun)
	if !ok {
		return false
	}
	switch {
	case pkg == "tmuxtest" && name == "StartServer":
		return true
	case name == "Command" || name == "CommandContext" || name == "LookPath" || (pkg == "testhost" && name == "Tool"):
		return hasTmuxLiteral(call.Args)
	}
	return false
}

func hasTmuxLiteral(args []ast.Expr) bool {
	for _, arg := range args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if value, err := strconv.Unquote(lit.Value); err == nil && value == "tmux" {
			return true
		}
	}
	return false
}

func callsSelector(root ast.Node, pkg, name string) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok {
			p, nm, isSel := selectorName(call.Fun)
			found = found || (isSel && p == pkg && nm == name)
		}
		return !found
	})
	return found
}

func selectorName(expr ast.Expr) (string, string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return ident.Name, sel.Sel.Name, true
}
