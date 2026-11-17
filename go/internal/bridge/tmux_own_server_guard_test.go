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

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
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

var tmuxIdentMarkers = map[string]bool{
	"requireTmux":         true,
	"itTmuxCtl":           true,
	"startOwnTmuxServer":  true,
	"ownTmuxServer":       true,
	"startContractServer": true,
}

var tmuxSelectorMarkers = map[string]bool{
	"tmuxtest.StartServer": true,
}

var tmuxBinaryCalls = map[string]bool{
	"testhost.Tool": true,
	"exec.LookPath": true,
}

const tmuxControllerLiteral = "execTmux"

func TestBridgeTests_EveryTmuxReachIsOutOfTheFloorBuild(t *testing.T) {
	t.Parallel()
	tag, ok := ciparity.EnvExclusiveTagOf("internal/bridge")
	if !ok {
		t.Fatal("the env-exclusive entry of internal/bridge declares no exclusion tag")
	}
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, path := range paths {
		offenders = append(offenders, tmuxReachesAtTheFloor(t, path, tag)...)
	}
	if len(offenders) > 0 {
		t.Errorf("%d bridge test file(s) reach tmux but still build under -tags %s; add !%s to the build constraint:\n%s",
			len(offenders), tag, tag, strings.Join(offenders, "\n"))
	}
}

func TestTmuxReachesAtTheFloor_FindsEveryMarkerAndSkipsTheExemptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, constraint, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(constraint+"package x\n\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		name, constraint, body string
		want                   []string
	}{
		{"require", "", "func TestA(t *testing.T) { requireTmux(t) }\n", []string{"requireTmux"}},
		{"tool", "", "func TestA(t *testing.T) { testhost.Tool(t, \"tmux\") }\n", []string{"testhost.Tool"}},
		{"toolother", "", "func TestA(t *testing.T) { testhost.Tool(t, \"git\") }\n", nil},
		{"lookpath", "", "func TestA(t *testing.T) { p, err := exec.LookPath(\"tmux\"); use(p, err) }\n", []string{"exec.LookPath"}},
		{"lookother", "", "func TestA(t *testing.T) { _, _ = exec.LookPath(\"git\") }\n", nil},
		{"lookskip", "", "func TestA(t *testing.T) {\n\tif _, err := exec.LookPath(\"tmux\"); err != nil {\n\t\tt.Skip(\"no tmux\")\n\t}\n}\n", nil},
		{"lookskipplus", "", "func TestA(t *testing.T) {\n\tif _, err := exec.LookPath(\"tmux\"); err != nil {\n\t\tt.Skip(\"no tmux\")\n\t\tuse(err)\n\t}\n}\n", []string{"exec.LookPath"}},
		{"literal", "", "func TestA(t *testing.T) { _ = execTmux{}.HasSession(nil, \"s\") }\n", []string{"execTmux{}"}},
		{"untyped", "", "var ctl = execTmux{}\n", []string{"execTmux{}"}},
		{"conversion", "", "func TestA(t *testing.T) {\n\tvar ctl TmuxController = execTmux{}\n\t_, _ = ctl.(Joined)\n}\n", nil},
		{"assertion", "", "func TestA(t *testing.T) { _, _ = ctl.(execTmux) }\n", nil},
		{"ctl", "", "func TestA(t *testing.T) { itTmuxCtl.KillSession(nil, \"s\") }\n", []string{"itTmuxCtl"}},
		{"start", "", "func TestA(t *testing.T) { _ = tmuxtest.StartServer(\"s\") }\n", []string{"tmuxtest.StartServer"}},
		{"startown", "", "func TestA(t *testing.T) { startOwnTmuxServer(t, \"s\", nil) }\n", []string{"startOwnTmuxServer"}},
		{"own", "", "func TestA(t *testing.T) { ownTmuxServer(t, \"s\", nil) }\n", []string{"ownTmuxServer"}},
		{"contract", "", "func TestA(t *testing.T) { startContractServer(t, \"s\") }\n", []string{"startContractServer"}},
		{"declared", "", "func requireTmux(t *testing.T) {}\nfunc TestA(t *testing.T) {}\n", nil},
		{"tagged", "//go:build integration && !notmux\n\n", "func TestA(t *testing.T) { requireTmux(t) }\n", nil},
		{"darwin", "//go:build integration && darwin && !notmux\n\n", "func TestA(t *testing.T) { requireTmux(t) }\n", nil},
		{"integrationonly", "//go:build integration\n\n", "func TestA(t *testing.T) { requireTmux(t) }\n", []string{"requireTmux"}},
		{"either", "//go:build !notmux || darwin\n\n", "func TestA(t *testing.T) { requireTmux(t) }\n", []string{"requireTmux"}},
		{"twice", "", "func TestA(t *testing.T) { requireTmux(t); requireTmux(t) }\n", []string{"requireTmux"}},
	}
	for _, tc := range cases {
		got := tmuxReachesAtTheFloor(t, write(tc.name+"_test.go", tc.constraint, tc.body), "notmux")
		var want []string
		for _, marker := range tc.want {
			want = append(want, tc.name+"_test.go:"+marker)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: found %q, want %q", tc.name, got, want)
		}
	}
}

func tmuxReachesAtTheFloor(t *testing.T, path, tag string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if leavesTheBuild(file, tag) {
		return nil
	}
	var offenders []string
	for _, marker := range tmuxMarkersIn(file) {
		offenders = append(offenders, filepath.Base(path)+":"+marker)
	}
	return offenders
}

func leavesTheBuild(file *ast.File, tag string) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			return false
		}
		for _, c := range group.List {
			if expr, err := constraint.Parse(c.Text); err == nil {
				return !expr.Eval(func(name string) bool { return name == tag }) && !expr.Eval(func(name string) bool { return name != "ignore" })
			}
		}
	}
	return false
}

func tmuxMarkersIn(file *ast.File) []string {
	var found []string
	note := func(marker string) {
		for _, seen := range found {
			if seen == marker {
				return
			}
		}
		found = append(found, marker)
	}
	exempt := map[ast.Node]bool{}
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			exempt[fd.Name] = true
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt:
			if call := lookPathInit(v); call != nil && onlySkips(v.Body) {
				exempt[call] = true
			}
		case *ast.ValueSpec:
			if v.Type != nil {
				for _, value := range v.Values {
					if lit, ok := value.(*ast.CompositeLit); ok {
						exempt[lit] = true
					}
				}
			}
		case *ast.Ident:
			if tmuxIdentMarkers[v.Name] && !exempt[v] {
				note(v.Name)
			}
		case *ast.SelectorExpr:
			if name := exprName(v); tmuxSelectorMarkers[name] {
				note(name)
			}
		case *ast.CallExpr:
			if name := exprName(v.Fun); tmuxBinaryCalls[name] && namesTmux(v) && !exempt[v] {
				note(name)
			}
		case *ast.CompositeLit:
			if exprName(v.Type) == tmuxControllerLiteral && len(v.Elts) == 0 && !exempt[v] {
				note(tmuxControllerLiteral + "{}")
			}
		}
		return true
	})
	return found
}

func namesTmux(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Value == strconv.Quote("tmux") {
			return true
		}
	}
	return false
}

func lookPathInit(stmt *ast.IfStmt) *ast.CallExpr {
	assign, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return nil
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || exprName(call.Fun) != "exec.LookPath" {
		return nil
	}
	return call
}

func onlySkips(body *ast.BlockStmt) bool {
	for _, stmt := range body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok || !strings.HasPrefix(exprName(call.Fun), "t.Skip") {
			return false
		}
	}
	return true
}
