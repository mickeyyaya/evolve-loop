package gatepins

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type shape func(body []ast.Stmt) bool

func refusesUnconfinedLaunch(body []ast.Stmt) bool {
	wrapped := ""
	for _, st := range body {
		if name := wrappedResultOf(st); name != "" {
			wrapped = name
			continue
		}
		ifs, ok := st.(*ast.IfStmt)
		if ok && wrapped != "" && callsWithLastArg(ifs.Cond, "sandboxRequiredButUnavailable", wrapped) && returnsName(ifs.Body, "ExitSafetyGate") {
			return true
		}
	}
	return false
}

func installsDefaultSandboxWrap(body []ast.Stmt) bool {
	for _, st := range body {
		ifs, ok := st.(*ast.IfStmt)
		if ok && isNilCheckOf(ifs.Cond, "SandboxWrap") && assignsCall(ifs.Body, "SandboxWrap", "defaultSandboxWrap") {
			return true
		}
	}
	return false
}

func passesRequireSandbox(body []ast.Stmt) bool {
	for _, st := range body {
		ifs, ok := st.(*ast.IfStmt)
		if ok && isSelector(ifs.Cond, "RequireSandbox") && holdsString(ifs.Body, "--require-sandbox") {
			return true
		}
	}
	return false
}

func wrappedResultOf(st ast.Stmt) string {
	as, ok := st.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 3 || len(as.Rhs) != 1 || !isCallTo(as.Rhs[0], "wrapHeadlessInvocation") {
		return ""
	}
	if id, ok := as.Lhs[2].(*ast.Ident); ok && id.Name != "_" {
		return id.Name
	}
	return ""
}

func isCallTo(e ast.Expr, name string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

func callsWithLastArg(e ast.Expr, name, arg string) bool {
	if !isCallTo(e, name) {
		return false
	}
	args := e.(*ast.CallExpr).Args
	if len(args) == 0 {
		return false
	}
	id, ok := args[len(args)-1].(*ast.Ident)
	return ok && id.Name == arg
}

func returnsName(block *ast.BlockStmt, name string) bool {
	for _, st := range block.List {
		ret, ok := st.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		if id, ok := ret.Results[0].(*ast.Ident); ok && id.Name == name {
			return true
		}
	}
	return false
}

func isNilCheckOf(e ast.Expr, field string) bool {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return false
	}
	id, ok := bin.Y.(*ast.Ident)
	return ok && id.Name == "nil" && isSelector(bin.X, field)
}

func isSelector(e ast.Expr, field string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == field
}

func assignsCall(block *ast.BlockStmt, field, callee string) bool {
	for _, st := range block.List {
		as, ok := st.(*ast.AssignStmt)
		if ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 && isSelector(as.Lhs[0], field) && isCallTo(as.Rhs[0], callee) {
			return true
		}
	}
	return false
}

func holdsString(block *ast.BlockStmt, text string) bool {
	found := false
	ast.Inspect(block, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Trim(lit.Value, "\"`") == text {
			found = true
		}
		return !found
	})
	return found
}

func holds(path, function string, want shape) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return false, err
	}
	fn := findFunc(file, function)
	if fn == nil || fn.Body == nil {
		return false, fmt.Errorf("function %s not found in %s", function, path)
	}
	return want(fn.Body.List), nil
}

func findFunc(file *ast.File, function string) *ast.FuncDecl {
	recv, name, isMethod := strings.Cut(function, ".")
	if !isMethod {
		recv, name = "", function
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && receiverName(fn) == recv {
			return fn
		}
	}
	return nil
}

func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func writeGo(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.go")
	if err := os.WriteFile(path, []byte("package p\n\n"+src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const refusal = `name, args, wrapped := wrapHeadlessInvocation(deps, cfg, bin, args)
	if sandboxRequiredButUnavailable(deps, cfg, wrapped) {
		return ExitSafetyGate, nil
	}`

func TestRefusesUnconfinedLaunch_HoldsOnlyTheRealRefusal(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"the real refusal", refusal, true},
		{"the result is discarded", "name, args, wrapped := wrapHeadlessInvocation(deps, cfg, bin, args)\n\t_ = sandboxRequiredButUnavailable(deps, cfg, wrapped)", false},
		{"the wrapped result is replaced by true", strings.Replace(refusal, "cfg, wrapped)", "cfg, true)", 1), false},
		{"the refusal sits in a dead branch", "if false {\n\t" + refusal + "\n\t}", false},
		{"the refusal returns no safety gate", strings.Replace(refusal, "return ExitSafetyGate, nil", "return 0, nil", 1), false},
		{"the wrapped result is dropped", strings.Replace(refusal, "name, args, wrapped :=", "name, args, _ :=", 1), false},
		{"the refusal call has no argument", strings.Replace(refusal, "sandboxRequiredButUnavailable(deps, cfg, wrapped)", "sandboxRequiredButUnavailable()", 1), false},
		{"no wrap at all", "if sandboxRequiredButUnavailable(deps, cfg, wrapped) {\n\t\treturn ExitSafetyGate, nil\n\t}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGo(t, "type d struct{}\n\nfunc (*d) Launch() (int, error) {\n\t"+tc.body+"\n\treturn 0, nil\n}\n")
			got, err := holds(path, "d.Launch", refusesUnconfinedLaunch)
			if err != nil || got != tc.want {
				t.Errorf("holds = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestInstallsDefaultSandboxWrap_HoldsOnlyTheNilCheckedDefault(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"the real default", "if d.SandboxWrap == nil {\n\t\td.SandboxWrap = defaultSandboxWrap(d)\n\t}", true},
		{"a stub replaces the default", "if d.SandboxWrap == nil {\n\t\td.SandboxWrap = noSandbox(d)\n\t}", false},
		{"the guard checks another field", "if d.Other == nil {\n\t\td.SandboxWrap = defaultSandboxWrap(d)\n\t}", false},
		{"the guard is not a nil check", "if d.SandboxWrap != nil {\n\t\td.SandboxWrap = defaultSandboxWrap(d)\n\t}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGo(t, "type Deps struct{}\n\nfunc (d Deps) withDefaults() Deps {\n\t"+tc.body+"\n\treturn d\n}\n")
			got, err := holds(path, "Deps.withDefaults", installsDefaultSandboxWrap)
			if err != nil || got != tc.want {
				t.Errorf("holds = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestPassesRequireSandbox_HoldsOnlyTheFlagUnderTheRequirement(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"the real flag", "if req.RequireSandbox {\n\t\targs = append(args, \"--require-sandbox\")\n\t}", true},
		{"the flag is gone", "if req.RequireSandbox {\n\t\targs = append(args, \"--x\")\n\t}", false},
		{"the requirement is gone", "if req.Other {\n\t\targs = append(args, \"--require-sandbox\")\n\t}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGo(t, "func launchArgs(req r) []string {\n\tvar args []string\n\t"+tc.body+"\n\treturn args\n}\n")
			got, err := holds(path, "launchArgs", passesRequireSandbox)
			if err != nil || got != tc.want {
				t.Errorf("holds = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestHolds_AnAbsentFunctionAndABadFileFail(t *testing.T) {
	path := writeGo(t, "func other() {}\n\ntype T[K any] struct{}\n\nfunc (T[K]) Launch() {}\n")
	if _, err := holds(path, "d.Launch", refusesUnconfinedLaunch); err == nil {
		t.Error("an absent function must fail the pin, not pass it")
	}
	if _, err := holds(path, "T.Launch", refusesUnconfinedLaunch); err == nil {
		t.Error("a generic receiver is not resolved, so the pin must fail loudly")
	}
	if _, err := holds(filepath.Join(t.TempDir(), "absent.go"), "f", refusesUnconfinedLaunch); err == nil {
		t.Error("an absent file must fail the pin, not pass it")
	}
}

func referencedNames(path string) (map[string]bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			names[id.Name] = true
		}
		return true
	})
	return names, nil
}

func firstOf(names map[string]bool, symbols []string) string {
	for _, s := range symbols {
		if names[s] {
			return s
		}
	}
	return ""
}

func TestReferencedNames_FindsAFencedSymbolAndFailsOnABadFile(t *testing.T) {
	path := writeGo(t, "func f() { _ = validateEnvPassthrough }\n")
	names, err := referencedNames(path)
	if err != nil || firstOf(names, []string{"absent", "validateEnvPassthrough"}) != "validateEnvPassthrough" {
		t.Errorf("names = %v, %v, want validateEnvPassthrough found", names, err)
	}
	if firstOf(names, []string{"absent"}) != "" {
		t.Error("a symbol that the file never names must not be found")
	}
	if _, err := referencedNames(filepath.Join(t.TempDir(), "absent.go")); err == nil {
		t.Error("an absent file must be an error")
	}
}
