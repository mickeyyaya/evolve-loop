package policy_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var paramPackages = []string{
	"internal/directives",
	"internal/policy",
	"internal/quotareset",
}

func findModuleRoot(t *testing.T, start string) string {
	t.Helper()
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from %s", start)
		}
		dir = parent
	}
}

func TestParamPackages_EnvAgnostic(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	moduleRoot := findModuleRoot(t, filepath.Dir(thisFile))
	banned := map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true}

	for _, rel := range paramPackages {
		dir := filepath.Join(moduleRoot, rel)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read enrolled param package %s: %v", rel, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, isIdent := sel.X.(*ast.Ident)
				if !isIdent || pkg.Name != "os" || !banned[sel.Sel.Name] {
					return true
				}
				t.Errorf("%s:%d: enrolled parameter package %q must be environment-agnostic; found os.%s — derive behavior from typed input parameters, not the system environment",
					path, fset.Position(sel.Pos()).Line, rel, sel.Sel.Name)
				return true
			})
		}
	}
}
