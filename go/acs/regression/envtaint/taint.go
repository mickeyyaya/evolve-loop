//go:build acs

// Package envtaint is the constant-folding + env-source taint harness for the
// honest flag-metric gate (Pillar 2 of ADR-0064, the pipeline-integrity
// boundary).
//
// The existing flag-reader guard (go/acs/regression/flagreaders) scans with
// go/ast and strconv.Unquote, so it only sees *ast.BasicLit string literals.
// Cycle 20 gamed that by writing os.Getenv("EVOLVE_" + "WORKTREE_BASE"): the
// dial kept working byte-for-byte while the literal "EVOLVE_WORKTREE_BASE"
// vanished from every grep/AST scan, so the registry row could be deleted with
// no guard objecting.
//
// This harness type-checks source with go/types instead. The type-checker runs
// the language spec's constant folding, so "EVOLVE_" + "WORKTREE_BASE" is
// already the single string value "EVOLVE_WORKTREE_BASE" by the time we inspect
// it — the split-const dodge is transparent. It also lets us tell a
// compile-time-constant os.Getenv argument (a real, countable operator dial)
// from a dynamic one (a non-constant key, not a fixed dial).
//
// S0 (this file) is the scaffold: load/fold + os.Getenv/LookupEnv classification.
// Later slices build the carrier read-set R and the derived registry on top.
package envtaint

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
)

// Harness holds a parsed and type-checked single-file Go package, with the
// go/types information needed to read folded constant values and classify call
// arguments.
type Harness struct {
	fset *token.FileSet
	file *ast.File
	pkg  *types.Package
	info *types.Info
}

// Load parses and type-checks a single-file Go source snippet. Type-checking
// performs constant folding, so callers can read folded constant values and
// distinguish constant from dynamic expressions. Imports (e.g. "os") are
// resolved from compiled package export data via go/importer.ForCompiler("gc")
// — fast, and no golang.org/x/tools dependency.
func Load(src string) (*Harness, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := &types.Config{Importer: importer.ForCompiler(fset, "gc", nil)}
	pkg, err := conf.Check(file.Name.Name, fset, []*ast.File{file}, info)
	if err != nil {
		return nil, fmt.Errorf("typecheck: %w", err)
	}
	return &Harness{fset: fset, file: file, pkg: pkg, info: info}, nil
}

// ConstStringValue returns the constant-folded string value of a package-level
// constant by name. ok is false when no such constant exists or it is not a
// string. Because the type-checker already folded the initializer, a
// split-const initializer like "EVOLVE_" + "X" is reported as "EVOLVE_X".
func (h *Harness) ConstStringValue(name string) (string, bool) {
	c, ok := h.pkg.Scope().Lookup(name).(*types.Const)
	if !ok {
		return "", false
	}
	v := c.Val()
	if v.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(v), true
}

// GetenvCall describes one os.Getenv / os.LookupEnv call site.
type GetenvCall struct {
	Key      string // folded key; meaningful only when Constant is true
	Constant bool   // true iff the argument is a compile-time string constant
}

// GetenvCalls returns every os.Getenv / os.LookupEnv call in the snippet. A
// call whose argument folds to a string constant is reported with Constant=true
// and the folded Key; a dynamic argument is reported with Constant=false and an
// empty Key.
func (h *Harness) GetenvCalls() []GetenvCall {
	var calls []GetenvCall
	ast.Inspect(h.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !h.isEnvSource(call.Fun) {
			return true
		}
		if tv := h.info.Types[call.Args[0]]; tv.Value != nil && tv.Value.Kind() == constant.String {
			calls = append(calls, GetenvCall{Key: constant.StringVal(tv.Value), Constant: true})
		} else {
			calls = append(calls, GetenvCall{Constant: false})
		}
		return true
	})
	return calls
}

// isEnvSource reports whether fun is a call to os.Getenv or os.LookupEnv,
// resolved through the type-checker rather than by matching the text "os.Getenv"
// — so a shadowed or aliased identifier cannot spoof an env source.
func (h *Harness) isEnvSource(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := h.info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os" {
		return false
	}
	switch fn.Name() {
	case "Getenv", "LookupEnv":
		return true
	default:
		return false
	}
}
