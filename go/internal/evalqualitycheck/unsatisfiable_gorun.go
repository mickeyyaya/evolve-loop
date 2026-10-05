package evalqualitycheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

var commandRunners = map[string]bool{"Run": true, "Output": true, "CombinedOutput": true, "Wait": true}

func goRunExitCodeVerdict(n ast.Node, scopes scopeStack) (verdict, bool) {
	recv, code, ok := exitCodeComparedTo(n)
	if !ok {
		return verdict{}, false
	}
	result, isGoRun := goRunResultOf(scopes, recv)
	if !isGoRun || result.reachableExitCode(code) {
		return verdict{}, false
	}
	return verdict{UnsatisfiableKindGoRunExitCode, fmt.Sprintf("go run reports any non-zero exit of the program it runs as 1, so the program's exit code %d never reaches ExitCode(); build the program with go build and run the built binary", code)}, true
}

func (b binding) reachableExitCode(code int64) bool {
	return code == 0 || code == 1 || (code == 2 && b.goFlagsGiven)
}

func goRunResultOf(scopes scopeStack, recv ast.Expr) (binding, bool) {
	if b := scopes.valueOf(recv); b.kind == kindGoRunExitError {
		return b, true
	}
	sel, ok := ast.Unparen(recv).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ProcessState" {
		return binding{}, false
	}
	b := scopes.valueOf(sel.X)
	return b, b.kind == kindGoRunCommand
}

func goRunCommand(expr ast.Expr) (binding, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return binding{}, false
	}
	argv, _, isExec := execArgv(call, nil)
	if !isExec || len(argv) < 2 || argv[0] != "go" || argv[1] != "run" {
		return binding{}, false
	}
	goFlagsGiven := len(argv) > 2 && (argv[2] == "" || strings.HasPrefix(argv[2], "-"))
	return binding{kind: kindGoRunCommand, goFlagsGiven: goFlagsGiven}, true
}

func ranGoRunCommand(s scopeStack, expr ast.Expr) (binding, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return binding{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !commandRunners[sel.Sel.Name] {
		return binding{}, false
	}
	if cmd, ok := goRunCommand(ast.Unparen(sel.X)); ok {
		return cmd, true
	}
	cmd := s.valueOf(sel.X)
	return cmd, cmd.kind == kindGoRunCommand
}

func assertedExitError(s scopeStack, expr ast.Expr) (binding, bool) {
	assertion, ok := expr.(*ast.TypeAssertExpr)
	if !ok || !isExitErrorPointer(assertion.Type) {
		return binding{}, false
	}
	err := s.valueOf(assertion.X)
	return err, err.kind == kindGoRunError
}

func bindErrorsAs(s scopeStack, call *ast.CallExpr) {
	if !isPkgCall(call, "errors", "As") || len(call.Args) != 2 {
		return
	}
	err := s.valueOf(call.Args[0])
	target, ok := call.Args[1].(*ast.UnaryExpr)
	if err.kind != kindGoRunError || !ok || target.Op != token.AND {
		return
	}
	if id, ok := target.X.(*ast.Ident); ok {
		s.assign(id, binding{kind: kindGoRunExitError, goFlagsGiven: err.goFlagsGiven})
	}
}

func isExitErrorPointer(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ExitError" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "exec"
}

func exitCodeComparedTo(n ast.Node) (ast.Expr, int64, bool) {
	bin, ok := n.(*ast.BinaryExpr)
	if !ok || (bin.Op != token.EQL && bin.Op != token.NEQ) {
		return nil, 0, false
	}
	for _, pair := range [][2]ast.Expr{{bin.X, bin.Y}, {bin.Y, bin.X}} {
		recv, isExitCode := exitCodeReceiver(pair[0])
		code, isLiteral := intLiteral(pair[1])
		if isExitCode && isLiteral {
			return recv, code, true
		}
	}
	return nil, 0, false
}

func exitCodeReceiver(expr ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ExitCode" {
		return nil, false
	}
	return sel.X, true
}

func intLiteral(expr ast.Expr) (int64, bool) {
	lit, ok := ast.Unparen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	v, err := strconv.ParseInt(lit.Value, 0, 64)
	return v, err == nil
}
