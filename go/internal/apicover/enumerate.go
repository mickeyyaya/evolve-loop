package apicover

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// SymbolKind classifies an exported declaration.
type SymbolKind int

// Symbol kinds.
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

// Symbol is one exported declaration discovered by Enumerate. Methods are keyed
// "ReceiverType.Method"; everything else is keyed by its bare exported name.
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

// Enumerate returns the exported symbols of the package rooted at dir, skipping
// _test.go files and any file excluded by the default build context (i.e.
// integration/e2e/acs-tagged or platform-mismatched files). Reusing
// build.Default.MatchFile means the measured surface matches exactly what an
// untagged `go test` compiles.
func Enumerate(dir string) ([]Symbol, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var syms []Symbol
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		match, err := build.Default.MatchFile(dir, name)
		if err != nil {
			return nil, err
		}
		if !match {
			continue // integration/e2e/acs-tagged or platform-mismatched
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

// exportedSymbols walks one parsed file and returns its exported declarations.
// An //apicover:ignore directive with an empty reason is a hard error.
func exportedSymbols(fset *token.FileSet, f *ast.File, file string) ([]Symbol, error) {
	pkg := f.Name.Name
	var out []Symbol
	var firstErr error
	add := func(name string, kind SymbolKind, pos token.Pos, doc *ast.CommentGroup) {
		ignored, reason, err := parseIgnore(doc)
		if err != nil {
			// A malformed directive still signals intent-to-ignore: bucket it as
			// ignored so it never surfaces as a spurious UNCOVERED line. Enumerate
			// also returns firstErr, so callers can fail hard if they choose.
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
			if !d.Name.IsExported() {
				continue
			}
			if d.Recv != nil {
				recv := recvTypeName(d.Recv)
				if recv == "" || !ast.IsExported(recv) {
					continue // method on an unexported type is not public API
				}
				add(recv+"."+d.Name.Name, KindMethod, d.Pos(), d.Doc)
				continue
			}
			add(d.Name.Name, KindFunc, d.Pos(), d.Doc)
		case *ast.GenDecl:
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
	}
	return out, firstErr
}

// recvTypeName extracts the receiver's bare type name, unwrapping pointer and
// generic-instantiation receivers (*T, T[P], *T[P]).
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
	case *ast.IndexExpr: // generic receiver: T[P]
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr: // generic receiver: T[P, Q]
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
