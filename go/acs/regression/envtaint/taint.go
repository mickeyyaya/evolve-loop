//go:build acs

// Package envtaint is the constant-folding + env-source taint harness for
// the honest flag-metric gate (Pillar 2 of ADR-0064, the pipeline-integrity
// boundary).
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

type Harness struct {
	fset *token.FileSet
	file *ast.File
	pkg  *types.Package
	info *types.Info
}

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

type GetenvCall struct {
	Key      string
	Constant bool
}

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
