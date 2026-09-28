// Package testmainexit finds TestMain functions whose deferred cleanup never runs: os.Exit skips defers.
package testmainexit

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func SkippedDefers(path string) ([]int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var lines []int
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "TestMain" || fn.Body == nil {
			continue
		}
		defers, exits := scan(fn.Body)
		if !exits {
			continue
		}
		for _, d := range defers {
			lines = append(lines, fset.Position(d.Pos()).Line)
		}
	}
	return lines, nil
}

func scan(body *ast.BlockStmt) (defers []*ast.DeferStmt, exits bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			defers = append(defers, n)
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exit" {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" {
					exits = true
				}
			}
		}
		return true
	})
	return defers, exits
}
