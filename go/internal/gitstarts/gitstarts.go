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
	"strconv"
	"strings"
)

func Count(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("gitstarts: read %s: %w", dir, err)
	}
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
				total += callCost(call)
			}
			return true
		})
	}
	return total, nil
}

func callCost(call *ast.CallExpr) int {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "gittest" {
			switch sel.Sel.Name {
			case "Fixture", "Bare", "At":
				return 5
			case "Clone":
				return 1
			case "FromTemplate", "BareFromTemplate":
				return 0
			}
		}
		if sel.Sel.Name == "Git" {
			return 1
		}
	}
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if s, err := strconv.Unquote(lit.Value); err == nil && s == "git" {
			return 1
		}
	}
	return 0
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
