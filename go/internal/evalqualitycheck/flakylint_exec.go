// flakylint_exec.go — subprocess-argv analysis for the flaky-shape lint
// (flakylint.go holds the patterns and their reporters; this file holds the
// question "what argv position did this string actually reach?").
//
// Split out purely for the 800-LOC file ceiling; the two halves are one lint and
// share its package doc. Everything here is a pure AST reader — no findings are
// produced in this file, only the evidence the pattern rules consult.

package evalqualitycheck

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

// --- M4/M6: argv-position index over a function's exec calls -----------------

// execPatternIndex records, for one function body, where each package pattern
// appeared across the argvs of ALL recognized subprocess constructors
// (execConstructors). It is what makes the suite-scope rule argv-position aware
// instead of a blanket literal scan.
type execPatternIndex struct {
	inGoTest    map[string]bool // reached a `go test` argv
	inAnyExec   map[string]bool // reached ANY exec argv (go test included)
	wideGoTest  map[string]bool // reached a `go test` argv that carried NO -run
	hasNarrowed map[string]bool // reached a `go test` argv that carried -run/-run=
}

// suiteScopeApplies decides whether s is in a position where a suite-scope
// finding is a true claim: it reached a `go test` argv (direct evidence), or it
// reached no exec argv at all (helper-mediated / pure data — unresolvable, so
// the advisory note stands, the safe direction). A pattern that reached only
// non-test subprocesses is a compile or a search, never a suite.
func (ix execPatternIndex) suiteScopeApplies(s string) bool {
	return ix.inGoTest[s] || !ix.inAnyExec[s]
}

// narrowedWithRun reports whether EVERY `go test` argv carrying s also carried
// -run/-run=. Requiring "every" (not "any") keeps the safe direction: one wide
// invocation of the same pattern elsewhere in the function keeps the finding.
//
// PROMOTION PRECONDITIONS (review MEDIUM — this is presence-only by design at
// ADVISORY stage, where a false claim costs author trust and suppression costs
// nothing; both must be closed BEFORE any promotion to enforce, or the gate is
// evadable):
//  1. The -run VALUE is never inspected, so `-run .`, `-run .*`, `-run ^Test`
//     and an empty value all read as narrowing. Reject trivial values.
//  2. A package pattern built at runtime (fmt.Sprintf, os.Getenv) never reaches
//     the literal scan at all, so string construction alone evades the lint.
//     Treat an unresolvable pattern as NOT narrowed.
//
// Bounded today: IsRecursive findings are checked before this suppression and so
// are never suppressible, and acssuite's run-time scope demotion still fires.
func (ix execPatternIndex) narrowedWithRun(s string) bool {
	return ix.hasNarrowed[s] && !ix.wideGoTest[s]
}

// indexExecPatterns records every package pattern's argv position(s) across the
// exec constructors reachable from fn — its own body, plus ONE level into
// same-package helpers it calls, with the call's arguments bound to the helper's
// parameter names.
//
// The helper hop is not a nicety, it is what makes the rule true on the real
// corpus. The canonical house shape puts the package pattern in a const the TEST
// function names and the `-run` narrowing in a shared helper:
//
//	const corePkg = "github.com/.../internal/core"
//	func TestC1034_001_X(t *testing.T) { assertDefaultSuiteTestsPass(t, corePkg, "TestAssembler_X") }
//	func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
//	        acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
//	}
//
// Body-only indexing saw no exec argv in the test function, fell into the
// "unresolvable, keep the note" branch, and printed "narrow the invocation with
// -run" at code that already does exactly that — 142 of 297 corpus findings
// (48%), measured. A lint that prints a false claim is one authors learn to
// ignore, so resolving the hop is the fix; the truly unresolvable case (a pattern
// built by concatenation, or handed to a helper two levels down) keeps its note.
func indexExecPatterns(fn *ast.FuncDecl, consts map[string]string, helpers map[string]*ast.FuncDecl) execPatternIndex {
	ix := execPatternIndex{
		inGoTest:    map[string]bool{},
		inAnyExec:   map[string]bool{},
		wideGoTest:  map[string]bool{},
		hasNarrowed: map[string]bool{},
	}
	ix.addFrom(fn.Body, consts)
	// Depth 1 exactly: a helper's own helper calls are NOT followed, so this
	// terminates unconditionally (no recursion, no visited set needed) — and a
	// self-call cannot loop either.
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

// addFrom folds one body's exec argvs into the index, resolving string args
// through scope (package consts, plus bound helper params on the helper hop).
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

// bindParams returns scope extended with helper's parameter names bound to the
// call site's resolvable string arguments, positionally. Binding stops at a
// variadic parameter (many args, one name — not a string binding). The result is
// a COPY: helper bindings must never leak into the package-const map the
// finding-generating scan reads.
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

// argvHasRunFilter reports whether a `go test` argv carries a -run selector
// (M6): a correctly narrowed invocation runs a handful of tests, not the suite,
// so its known-slow-package cost estimate no longer holds.
func argvHasRunFilter(argv []string) bool {
	for _, a := range argv {
		if a == "-run" || a == "--run" || strings.HasPrefix(a, "-run=") || strings.HasPrefix(a, "--run=") {
			return true
		}
	}
	return false
}

// --- exec-call plumbing ------------------------------------------------------

// execConstructor describes one recognized way an ACS predicate spawns a
// subprocess: the pkg.Fn selector, whether a leading context arg must be
// skipped, and whether the constructor BINDS that context to the child's
// lifetime (which is what makes load-generation reaped rather than orphaned).
type execConstructor struct{ skipCtxArg, ctxBound bool }

// execConstructors is the recognized set, keyed "pkg.Fn". acsassert
// .SubprocessOutput belongs here on evidence, not on taste: it is a thin
// exec.Command(name, args...) wrapper and it is what the corpus actually uses —
// 198 of 282 historical acs dirs call it, against 20 that reach for exec.Command.
// Omitting it made the argv-position rule (M4) technically implemented and
// practically inert, because virtually every real `go vet ./...` / `go build`
// predicate went through it and so landed in the "reached no exec argv at all"
// bucket that keeps its finding. It carries no context, so its load-generation
// really is un-reaped (the 8-cores-for-9-hours class).
var execConstructors = map[string]execConstructor{
	"exec.Command":               {},
	"exec.CommandContext":        {skipCtxArg: true, ctxBound: true},
	"acsassert.SubprocessOutput": {},
}

// execArgv extracts the literal/const-resolved argv of a recognized subprocess
// constructor (a leading ctx arg is skipped); non-literal args become "" so
// positions stay aligned. hasCtx reports whether the child's lifetime is bound
// to a context. ok=false when call is not an exec constructor.
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

// resolveStringExpr returns a string literal's value, a package-level string
// const's value, or "" for anything dynamic.
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

// dirAnchoredVars returns the names assigned a `.Dir` in body (cmd.Dir = x —
// the house repo-anchoring idiom, equivalent to git -C).
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

// execAssignNames maps each call expression appearing as a direct RHS of an
// assignment to its LHS ident name (`cmd := exec.Command(...)` → cmd), so the
// git rule can honor a later cmd.Dir assignment.
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

// isPkgCall reports whether call is pkg.name(...) on a plain package ident.
func isPkgCall(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg
}

// containsArg reports whether args contains the exact literal want.
func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// argsContainStringLit reports whether want appears as a string literal (or a
// resolvable package const) ANYWHERE inside the given arg expressions —
// including nested composites like append([]string{"-C", dir}, args...).
// Presence can only UNFLAG (anchor), so deep matching is the safe direction.
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
