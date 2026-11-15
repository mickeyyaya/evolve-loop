package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const sharedModuleLoaderFile = "module_scan_loader_test.go"

var moduleRootNames = []string{"moduleRoot", "moduleDir", "goRoot"}

func directModuleWalks(t *testing.T, name string, src []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	moduleNames := moduleRootAliases(file)
	var walks []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		root := walkRootIndex(call.Fun)
		if root < 0 || root >= len(call.Args) {
			return true
		}
		if isModuleRootExpr(call.Args[root], moduleNames) {
			walks = append(walks, name+":"+strconv.Itoa(len(walks)+1))
		}
		return true
	})
	return walks
}

func walkRootIndex(fun ast.Expr) int {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return -1
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return -1
	}
	switch pkg.Name + "." + sel.Sel.Name {
	case "filepath.Walk", "filepath.WalkDir":
		return 0
	case "fs.WalkDir":
		return 1
	}
	return -1
}

func moduleRootAliases(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, name := range moduleRootNames {
		names[name] = true
	}
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			rhs := assign.Rhs[0]
			if len(assign.Rhs) == len(assign.Lhs) {
				rhs = assign.Rhs[i]
			} else if i > 0 {
				break
			}
			if id, ok := lhs.(*ast.Ident); ok && isModuleRootExpr(rhs, names) {
				names[id.Name] = true
			}
		}
		return true
	})
	return names
}

func isModuleRootExpr(expr ast.Expr, names map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return names[e.Name]
	case *ast.CallExpr:
		if isSelector(e.Fun, "filepath", "Abs") && len(e.Args) == 1 {
			return isModuleRootExpr(e.Args[0], names)
		}
		return isSelector(e.Fun, "filepath", "Join") && len(e.Args) >= 2 && allParentSegments(e.Args)
	}
	return false
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func allParentSegments(args []ast.Expr) bool {
	for _, arg := range args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || lit.Value != `".."` {
			return false
		}
	}
	return true
}

func TestModuleScanGuard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"a walk of the module root is a direct walk", `package p
import ("os";"path/filepath")
func f() { moduleRoot := filepath.Join("..", ".."); filepath.WalkDir(moduleRoot, func(string, os.DirEntry, error) error { return nil }) }`, 1},
		{"a walk of the Go root with filepath.Walk is a direct walk", `package p
import "path/filepath"
func f(goRoot string) { filepath.Walk(goRoot, nil) }`, 1},
		{"an fs.WalkDir of a module directory parameter is a direct walk", `package p
import ("io/fs";"os")
func f(moduleDir string) { fs.WalkDir(os.DirFS("."), moduleDir, nil); fs.WalkDir(os.DirFS(moduleDir), ".", nil) }`, 1},
		{"a walk of a renamed alias of the module root is a direct walk", `package p
import "path/filepath"
func f() { tree := filepath.Join("..", ".."); filepath.WalkDir(tree, nil) }`, 1},
		{"a walk of an absolute module root is a direct walk", `package p
import "path/filepath"
func f() { top, _ := filepath.Abs(filepath.Join("..", "..")); filepath.WalkDir(top, nil) }`, 1},
		{"a walk of a temporary directory is not a module walk", `package p
import "path/filepath"
func f(dir string) { filepath.WalkDir(dir, nil); filepath.Walk(filepath.Join(dir, "inbox"), nil) }`, 0},
		{"a walk of a literal two-level parent join is a direct walk", `package p
import "path/filepath"
func f() { filepath.WalkDir(filepath.Join("..", ".."), nil) }`, 1},
		{"a read of one module file is not a walk", `package p
import ("os";"path/filepath")
func f(moduleRoot string) { os.ReadFile(filepath.Join(moduleRoot, "go.mod")) }`, 0},
		{"a file without any walk reports nothing", `package p
func f() {}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := directModuleWalks(t, "fixture.go", []byte(tc.src)); len(got) != tc.want {
				t.Errorf("directModuleWalks = %v, want %d walk(s)", got, tc.want)
			}
		})
	}

	t.Run("the shared loader file exists", func(t *testing.T) {
		if _, err := os.Stat(sharedModuleLoaderFile); err != nil {
			t.Errorf("the home of the one module walk, %s, is missing: %v", sharedModuleLoaderFile, err)
		}
	})

	t.Run("no cmd/evolve test file walks the module without the shared loader", func(t *testing.T) {
		files, err := filepath.Glob("*_test.go")
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(files)
		var offenders []string
		for _, file := range files {
			if file == sharedModuleLoaderFile || file == "module_scan_guard_test.go" {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(src), "Walk") {
				continue
			}
			offenders = append(offenders, directModuleWalks(t, file, src)...)
		}
		if len(offenders) > 0 {
			t.Errorf("%d direct module walk(s) outside %s: %v", len(offenders), sharedModuleLoaderFile, offenders)
		}
	})
}
