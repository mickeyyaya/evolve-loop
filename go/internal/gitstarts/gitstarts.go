// Package gitstarts counts and verifies git process starts in Go test directories.
package gitstarts

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const repoMethodPrefix = "Repo."

func Count(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("gitstarts: read %s: %w", dir, err)
	}
	costs := gittest.StartCosts()
	fset := token.NewFileSet()
	total := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			return 0, fmt.Errorf("gitstarts: parse %s: %w", entry.Name(), err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				total += callCost(call, costs)
			}
			return true
		})
	}
	return total, nil
}

func callCost(call *ast.CallExpr, costs map[string]int) int {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "gittest" {
			if cost, ok := costs[sel.Sel.Name]; ok {
				return cost
			}
		} else if cost, ok := costs[repoMethodPrefix+sel.Sel.Name]; ok {
			return cost
		}
	}
	if hasGitStringArg(call.Args) {
		return 1
	}
	return 0
}

func hasGitStringArg(args []ast.Expr) bool {
	for _, arg := range args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if s, err := strconv.Unquote(lit.Value); err == nil && s == "git" {
			return true
		}
	}
	return false
}

func MissingCosts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gitstarts: read %s: %w", dir, err)
	}
	fset := token.NewFileSet()
	decls := map[string]*ast.FuncDecl{}
	var exported []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("gitstarts: parse %s: %w", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			key, isExported := declCostKey(fn)
			if key == "" {
				continue
			}
			decls[key] = fn
			if isExported {
				exported = append(exported, key)
			}
		}
	}
	costs := gittest.StartCosts()
	var missing []string
	for _, key := range exported {
		if _, ok := costs[key]; ok {
			continue
		}
		if funcStartsGit(key, decls, costs, map[string]bool{}) {
			missing = append(missing, key)
		}
	}
	slices.Sort(missing)
	return missing, nil
}

func declCostKey(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil {
		return fn.Name.Name, fn.Name.IsExported()
	}
	recv := receiverTypeName(fn.Recv.List[0].Type)
	if recv == "" {
		return "", false
	}
	return recv + "." + fn.Name.Name, ast.IsExported(recv) && fn.Name.IsExported()
}

func receiverTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func funcStartsGit(key string, decls map[string]*ast.FuncDecl, costs map[string]int, visiting map[string]bool) bool {
	if visiting[key] {
		return false
	}
	fn, ok := decls[key]
	if !ok || fn.Body == nil {
		return false
	}
	visiting[key] = true
	defer delete(visiting, key)
	starts := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if starts {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if directGitCall(call) {
			starts = true
			return false
		}
		for _, target := range intraPackageTargets(call) {
			if cost, ok := costs[target]; ok {
				if cost > 0 {
					starts = true
					return false
				}
				continue
			}
			if funcStartsGit(target, decls, costs, visiting) {
				starts = true
				return false
			}
		}
		return true
	})
	return starts
}

func directGitCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name == "Git" {
		return true
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || (id.Name != "exec" && id.Name != "sysexec") {
		return false
	}
	if sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext" {
		return false
	}
	return hasGitStringArg(call.Args)
}

func intraPackageTargets(call *ast.CallExpr) []string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return []string{fun.Name}
	case *ast.SelectorExpr:
		return []string{repoMethodPrefix + fun.Sel.Name}
	}
	return nil
}

func Verify(dir, recordedPath string) error {
	body, err := os.ReadFile(recordedPath)
	if err != nil {
		return fmt.Errorf("gitstarts: read recorded count %s: %w", recordedPath, err)
	}
	var doc struct {
		Recorded *int `json:"recorded"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("gitstarts: parse recorded count %s: %w", recordedPath, err)
	}
	if doc.Recorded == nil {
		return fmt.Errorf("gitstarts: %s missing recorded field", recordedPath)
	}
	if *doc.Recorded < 0 {
		return fmt.Errorf("gitstarts: %s recorded count %d is negative", recordedPath, *doc.Recorded)
	}
	got, err := Count(dir)
	if err != nil {
		return err
	}
	if got > *doc.Recorded {
		return fmt.Errorf("gitstarts: %s has %d git starts, exceeding recorded %d", dir, got, *doc.Recorded)
	}
	return nil
}
