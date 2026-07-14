package apicover

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// NamesReferencedInTests returns the set of identifier and selector names
// mentioned in the _test.go files of dir. It is deliberately broad (it does not
// resolve types): a method is "named" if its bare selector name appears. This is
// the AST half of the two-signal check — paired with executed coverage it
// distinguishes truly-untested symbols from named-but-0% false-greens.
func NamesReferencedInTests(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	names := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				names[x.Name] = true
			case *ast.SelectorExpr:
				names[x.Sel.Name] = true
			}
			return true
		})
	}
	return names, nil
}
