package evalqualitycheck

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

// execPatternIndex records which exec argvs each package pattern reached in one
// function, which makes the suite-scope rule argv-position aware.
type execPatternIndex struct {
	inGoTest    map[string]bool
	inAnyExec   map[string]bool // reached any exec argv, go test included
	wideGoTest  map[string]bool // reached a `go test` argv that carried NO -run
	hasNarrowed map[string]bool // reached a `go test` argv that carried -run/-run=
}

// suiteScopeApplies holds when s reached a go test argv or no exec argv at all:
// an unresolvable pattern keeps its advisory note, the safe direction.
func (ix execPatternIndex) suiteScopeApplies(s string) bool {
	return ix.inGoTest[s] || !ix.inAnyExec[s]
}

// narrowedWithRun requires every go test argv carrying s to carry -run, so one wide invocation keeps the finding.
// The -run value is not inspected, which is evadable; close that before promoting past advisory.
func (ix execPatternIndex) narrowedWithRun(s string) bool {
	return ix.hasNarrowed[s] && !ix.wideGoTest[s]
}

// indexExecPatterns indexes fn's own exec calls plus those one level into same-package
// helpers, with the call's args bound to the helper's params.
func indexExecPatterns(fn *ast.FuncDecl, consts map[string]string, helpers map[string]*ast.FuncDecl) execPatternIndex {
	ix := execPatternIndex{
		inGoTest:    map[string]bool{},
		inAnyExec:   map[string]bool{},
		wideGoTest:  map[string]bool{},
		hasNarrowed: map[string]bool{},
	}
	ix.addFrom(fn.Body, consts)
	// Depth 1 exactly: a helper's own calls are not followed, so the walk terminates without a visited set.
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

// bindParams returns a copy so helper bindings never leak into the package-const map
// the finding scan reads. Binding stops at a variadic param.
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

// execConstructor: ctxBound means the child dies with its context, which is what reaps load generation.
type execConstructor struct{ skipCtxArg, ctxBound bool }

// execConstructors is keyed "pkg.Fn". acsassert.SubprocessOutput is the corpus's usual
// exec wrapper; it binds no context, so its load generation is unreaped.
var execConstructors = map[string]execConstructor{
	"exec.Command":               {},
	"exec.CommandContext":        {skipCtxArg: true, ctxBound: true},
	"acsassert.SubprocessOutput": {},
}

// execArgv resolves a recognized exec call's argv; non-literal args become "" so positions stay aligned.
func execArgv(call *ast.CallExpr, consts map[string]string) (argv []string, hasCtx, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return nil, false, false
	}
	x, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return nil, false, false
	}
	ctor, known := execConstructors[x.Name+"."+sel.Sel.Name]
	if !known {
		return nil, false, false
	}
	hasCtx = ctor.ctxBound
	args := call.Args
	if ctor.skipCtxArg && len(args) > 0 {
		args = args[1:]
	}
	for _, a := range args {
		argv = append(argv, resolveStringExpr(a, consts))
	}
	return argv, hasCtx, true
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

// dirAnchoredVars names the vars assigned a .Dir, which anchors git the way -C does.
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

// execAssignNames maps each call assigned to an ident to that name, so the git rule can honor a later cmd.Dir.
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

// argsContainStringLit matches at any nesting depth; presence can only unflag, so deep matching is the safe direction.
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
