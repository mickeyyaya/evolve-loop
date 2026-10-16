package apicover

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type SymbolKind int

const (
	KindFunc SymbolKind = iota
	KindMethod
	KindType
	KindVar
	KindConst
)

func (k SymbolKind) String() string {
	switch k {
	case KindFunc:
		return "func"
	case KindMethod:
		return "method"
	case KindType:
		return "type"
	case KindVar:
		return "var"
	case KindConst:
		return "const"
	default:
		return "unknown"
	}
}

type Symbol struct {
	Pkg          string
	Name         string
	Kind         SymbolKind
	File         string
	Line         int
	HasDoc       bool
	Ignored      bool
	IgnoreReason string
}

func Enumerate(ctx context.Context, dir string) ([]Symbol, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var syms []Symbol
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		match, err := build.Default.MatchFile(dir, name)
		if err != nil {
			return nil, err
		}
		if !match {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		fileSyms, err := exportedSymbols(fset, f, name)
		if err != nil {
			return nil, err
		}
		syms = append(syms, fileSyms...)
	}
	return syms, nil
}

func exportedSymbols(fset *token.FileSet, f *ast.File, file string) ([]Symbol, error) {
	pkg := f.Name.Name
	var out []Symbol
	var firstErr error
	add := func(name string, kind SymbolKind, pos token.Pos, doc *ast.CommentGroup) {
		ignored, reason, err := parseIgnore(doc)
		if err != nil {
			ignored = true
			if firstErr == nil {
				firstErr = fmt.Errorf("%s:%d %s: %w", file, fset.Position(pos).Line, name, err)
			}
		}
		out = append(out, Symbol{
			Pkg:          pkg,
			Name:         name,
			Kind:         kind,
			File:         file,
			Line:         fset.Position(pos).Line,
			HasDoc:       doc != nil,
			Ignored:      ignored,
			IgnoreReason: reason,
		})
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			addFuncDecl(d, add)
		case *ast.GenDecl:
			addGenDecl(d, add)
		}
	}
	return out, firstErr
}

type symbolAdder func(name string, kind SymbolKind, pos token.Pos, doc *ast.CommentGroup)

func addFuncDecl(d *ast.FuncDecl, add symbolAdder) {
	if !d.Name.IsExported() {
		return
	}
	if d.Recv != nil {
		recv := recvTypeName(d.Recv)
		if recv == "" || !ast.IsExported(recv) {
			return
		}
		add(recv+"."+d.Name.Name, KindMethod, d.Pos(), d.Doc)
		return
	}
	add(d.Name.Name, KindFunc, d.Pos(), d.Doc)
}

func addGenDecl(d *ast.GenDecl, add symbolAdder) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if !s.Name.IsExported() {
				continue
			}
			add(s.Name.Name, KindType, s.Pos(), firstDoc(s.Doc, d.Doc))
		case *ast.ValueSpec:
			kind := KindVar
			if d.Tok == token.CONST {
				kind = KindConst
			}
			for _, n := range s.Names {
				if !n.IsExported() {
					continue
				}
				add(n.Name, kind, n.Pos(), firstDoc(s.Doc, d.Doc))
			}
		}
	}
}

func recvTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func firstDoc(groups ...*ast.CommentGroup) *ast.CommentGroup {
	for _, g := range groups {
		if g != nil {
			return g
		}
	}
	return nil
}
