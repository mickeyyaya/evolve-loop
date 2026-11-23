package evalqualitycheck

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

type execPatternIndex struct {
	inGoTest    map[string]bool
	inAnyExec   map[string]bool
	wideGoTest  map[string]bool
	hasNarrowed map[string]bool
}

func (ix execPatternIndex) suiteScopeApplies(s string) bool {
	return ix.inGoTest[s] || !ix.inAnyExec[s]
}

func (ix execPatternIndex) narrowedWithRun(s string) bool {
	return ix.hasNarrowed[s] && !ix.wideGoTest[s]
}

func indexExecPatterns(fn *ast.FuncDecl, consts map[string]string, helpers map[string]*ast.FuncDecl) execPatternIndex {
	ix := execPatternIndex{
		inGoTest:    map[string]bool{},
		inAnyExec:   map[string]bool{},
		wideGoTest:  map[string]bool{},
		hasNarrowed: map[string]bool{},
	}
	ix.addFrom(fn.Body, consts)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, isIdent := call.Fun.(*ast.Ident)
		if !isIdent || id.Name == fn.Name.Name {
			return true
		}
		helper, known := helpers[id.Name]
		if !known {
			return true
		}
		ix.addFrom(helper.Body, bindParams(helper, call, consts))
		return true
	})
	return ix
}

func (ix execPatternIndex) addFrom(body *ast.BlockStmt, scope map[string]string) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		argv, _, isExec := execArgv(call, scope)
		if !isExec {
			return true
		}
		isGoTest := len(argv) >= 2 && argv[0] == "go" && argv[1] == "test"
		narrowed := isGoTest && argvHasRunFilter(argv)
		for _, a := range argv {
			if !gopkgpattern.IsPackagePattern(a) {
				continue
			}
			ix.inAnyExec[a] = true
			if !isGoTest {
				continue
			}
			ix.inGoTest[a] = true
			if narrowed {
				ix.hasNarrowed[a] = true
			} else {
				ix.wideGoTest[a] = true
			}
		}
		return true
	})
}

func bindParams(helper *ast.FuncDecl, call *ast.CallExpr, scope map[string]string) map[string]string {
	out := make(map[string]string, len(scope)+4)
	for k, v := range scope {
		out[k] = v
	}
	if helper.Type == nil || helper.Type.Params == nil {
		return out
	}
	i := 0
	for _, field := range helper.Type.Params.List {
		if _, variadic := field.Type.(*ast.Ellipsis); variadic {
			return out
		}
		for _, name := range field.Names {
			if i < len(call.Args) {
				if v := resolveStringExpr(call.Args[i], scope); v != "" {
					out[name.Name] = v
				}
			}
			i++
		}
	}
	return out
}

func argvHasRunFilter(argv []string) bool {
	for _, a := range argv {
		if a == "-run" || a == "--run" || strings.HasPrefix(a, "-run=") || strings.HasPrefix(a, "--run=") {
			return true
		}
	}
	return false
}

type execConstructor struct {
	leadArgs int
	ctxBound bool
	bounded  bool
	dirArg   bool
}

var execConstructors = map[string]execConstructor{
	"exec.Command":                 {},
	"exec.CommandContext":          {leadArgs: 1, ctxBound: true},
	"acsassert.SubprocessOutput":   {ctxBound: true, bounded: true},
	"acsassert.SubprocessOutputIn": {leadArgs: 1, ctxBound: true, bounded: true, dirArg: true},
}

func execConstructorOf(call *ast.CallExpr) (execConstructor, bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return execConstructor{}, false
	}
	x, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return execConstructor{}, false
	}
	ctor, known := execConstructors[x.Name+"."+sel.Sel.Name]
	return ctor, known
}

func anchorsDirectory(call *ast.CallExpr) bool {
	ctor, known := execConstructorOf(call)
	return known && ctor.dirArg
}

func isBoundedSubprocessCall(call *ast.CallExpr) bool {
	ctor, known := execConstructorOf(call)
	return known && ctor.bounded
}

func execArgv(call *ast.CallExpr, consts map[string]string) (argv []string, hasCtx, ok bool) {
	ctor, known := execConstructorOf(call)
	if !known {
		return nil, false, false
	}
	args := call.Args[min(ctor.leadArgs, len(call.Args)):]
	for _, a := range args {
		argv = append(argv, resolveStringExpr(a, consts))
	}
	return argv, ctor.ctxBound, true
}

func resolveStringExpr(e ast.Expr, consts map[string]string) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return s
			}
		}
	case *ast.Ident:
		return consts[v.Name]
	}
	return ""
}

func dirAnchoredVars(body *ast.BlockStmt) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range as.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Dir" {
				if id, ok := sel.X.(*ast.Ident); ok {
					out[id.Name] = true
				}
			}
		}
		return true
	})
	return out
}

func execAssignNames(body *ast.BlockStmt) map[*ast.CallExpr]string {
	out := map[*ast.CallExpr]string{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			ce, ok := rhs.(*ast.CallExpr)
			if !ok || i >= len(as.Lhs) {
				continue
			}
			if id, ok := as.Lhs[i].(*ast.Ident); ok {
				out[ce] = id.Name
			}
		}
		return true
	})
	return out
}

func isPkgCall(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func argsContainStringLit(args []ast.Expr, want string, consts map[string]string) bool {
	found := false
	for _, a := range args {
		ast.Inspect(a, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					if s, err := strconv.Unquote(v.Value); err == nil && s == want {
						found = true
					}
				}
			case *ast.Ident:
				if consts[v.Name] == want {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}
