package landcmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func isAtomicPointerOfDeps(expr ast.Expr) bool {
	index, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	sel, ok := index.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Pointer" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "atomic" {
		return false
	}
	target := index.Index
	if star, ok := target.(*ast.StarExpr); ok {
		target = star.X
	}
	switch arg := target.(type) {
	case *ast.Ident:
		return strings.HasSuffix(strings.ToLower(arg.Name), "deps")
	case *ast.SelectorExpr:
		return strings.HasSuffix(strings.ToLower(arg.Sel.Name), "deps")
	}
	return false
}

func scanAtomicDepsGlobals(t *testing.T, cliRoot string) []string {
	t.Helper()
	var offenders []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(cliRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				hit := value.Type != nil && isAtomicPointerOfDeps(value.Type)
				for _, v := range value.Values {
					if lit, ok := v.(*ast.CompositeLit); ok && isAtomicPointerOfDeps(lit.Type) {
						hit = true
					}
				}
				if hit {
					for _, name := range value.Names {
						offenders = append(offenders, path+":"+name.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return offenders
}

func TestNoCLIPackageStoresDepsInAProcessGlobal(t *testing.T) {
	t.Parallel()
	cliRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if offenders := scanAtomicDepsGlobals(t, cliRoot); len(offenders) != 0 {
		t.Errorf("CLI packages must pass Deps down instead of storing them in a package-level atomic.Pointer: %v", offenders)
	}
}
