package apicover

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func NamesReferencedInTests(ctx context.Context, dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	names := map[string]bool{}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		var collect func(n ast.Node) bool
		collect = func(n ast.Node) bool {
			if rhs, declared, ok := blankBindingValues(n); ok {
				if declared != nil {
					ast.Inspect(declared, collect)
				}
				for _, v := range rhs {
					if !isBareReference(v) {
						ast.Inspect(v, collect)
					}
				}
				return false
			}
			switch x := n.(type) {
			case *ast.Ident:
				names[x.Name] = true
			case *ast.SelectorExpr:
				names[x.Sel.Name] = true
			}
			return true
		}
		ast.Inspect(f, collect)
	}
	return names, nil
}

func blankBindingValues(n ast.Node) ([]ast.Expr, ast.Expr, bool) {
	switch x := n.(type) {
	case *ast.ValueSpec:
		if allBlank(identExprs(x.Names)) && len(x.Values) > 0 {
			return x.Values, x.Type, true
		}
	case *ast.AssignStmt:
		if x.Tok == token.ASSIGN && allBlank(x.Lhs) {
			return x.Rhs, nil, true
		}
	}
	return nil, nil, false
}

func identExprs(ids []*ast.Ident) []ast.Expr {
	out := make([]ast.Expr, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func allBlank(exprs []ast.Expr) bool {
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); !ok || id.Name != "_" {
			return false
		}
	}
	return true
}

func isBareReference(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return isBareReference(x.X)
	case *ast.ParenExpr:
		return isBareReference(x.X)
	}
	return false
}
