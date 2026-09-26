package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A guard's decision comes from its constructor arguments and cycle state, never from an exported
// variable, so no shell an operator runs the suite from can turn a deny into an allow. The one read
// is $HOME, taken once by NewRole for the <home>/.claude always-safe rule.
func TestGuards_ReadNoEnvironmentButHome(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		osName := osImportName(f)
		if osName == "." {
			t.Errorf("%s dot-imports os, which hides every environment read from this check", name)
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if read, ok := environmentRead(n, osName); ok && !(name == "role.go" && read == `os.Getenv("HOME")`) {
				t.Errorf("%s: %s — guards take their settings from constructors, not the environment", fset.Position(n.Pos()), read)
			}
			return true
		})
	}
}

// osImportName is the identifier a file binds package os to: "os", an alias, ".", or "" when not imported.
func osImportName(f *ast.File) string {
	for _, imp := range f.Imports {
		if path, err := strconv.Unquote(imp.Path.Value); err != nil || path != "os" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "os"
	}
	return ""
}

func environmentRead(n ast.Node, osName string) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || osName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != osName || !environmentReaders[sel.Sel.Name] {
		return "", false
	}
	arg := ""
	if len(call.Args) == 1 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				arg = strconv.Quote(v)
			}
		}
	}
	return "os." + sel.Sel.Name + "(" + arg + ")", true
}

var environmentReaders = map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true}
