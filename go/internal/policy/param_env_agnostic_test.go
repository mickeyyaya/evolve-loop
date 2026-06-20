package policy_test

// Env-agnostic invariant guard. The operator directive for the flag→parameter
// conversion is that these packages must be agnostic to the system environment
// and derive behavior PURELY from their typed input parameters. This test parses
// every non-test .go file in internal/policy and internal/quotareset and fails
// if any references os.Getenv / os.LookupEnv / os.Environ. (File I/O such as
// os.ReadFile/os.Stat is fine — only global-environment reads are banned.)
// Currently clean; this locks it so a future edit can't silently reintroduce
// global-env coupling and the suite itself touches no environment.
//
// Scope note: matches the canonical `os` import name. An aliased `import o "os"`
// + `o.Getenv(...)` would evade the AST match — an unusual pattern caught in code
// review rather than here.

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

func TestParamPackages_EnvAgnostic(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	policyDir := filepath.Dir(thisFile)
	dirs := []string{policyDir, filepath.Join(policyDir, "..", "quotareset")}
	banned := map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
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
				t.Errorf("%s:%d: parameter package must be environment-agnostic; found os.%s — derive behavior from typed input parameters, not the system environment",
					path, fset.Position(sel.Pos()).Line, sel.Sel.Name)
				return true
			})
		}
	}
}
