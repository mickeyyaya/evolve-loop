// Package structure shares source-structure checks between package tests.
package structure

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxFuncLines = 50
	maxNesting   = 4
	maxFileLines = 800
)

// CheckLimits checks the non-test Go files directly in dir, regardless of build
// constraints. Functions must have fewer than 50 lines and at most four nested
// control statements; files must have fewer than 800 newline bytes. Function doc
// comments are excluded, while comments inside a function count toward its size.
func CheckLimits(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	var failures []error
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		if lines := strings.Count(string(src), "\n"); lines >= maxFileLines {
			failures = append(failures, fmt.Errorf("%s: %d lines ≥ %d", name, lines, maxFileLines))
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		failures = append(failures, checkFunctions(fset, name, file)...)
	}
	return errors.Join(failures...)
}

func checkFunctions(fset *token.FileSet, name string, file *ast.File) []error {
	var failures []error
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		lines := fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
		if lines >= maxFuncLines {
			failures = append(failures, fmt.Errorf("%s: %s is %d lines ≥ %d", name, fn.Name.Name, lines, maxFuncLines))
		}
		if depth := nesting(fn.Body, 0); depth > maxNesting {
			failures = append(failures, fmt.Errorf("%s: %s nests %d deep > %d", name, fn.Name.Name, depth, maxNesting))
		}
	}
	return failures
}

func nesting(node ast.Node, depth int) int {
	deepest := depth
	ast.Inspect(node, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if d := nesting(childBody(s), depth+1); d > deepest {
				deepest = d
			}
			if branch, ok := s.(*ast.IfStmt); ok && branch.Else != nil {
				if d := nesting(branch.Else, depth+1); d > deepest {
					deepest = d
				}
			}
			return false
		}
		return true
	})
	return deepest
}

func childBody(s ast.Node) ast.Node {
	switch v := s.(type) {
	case *ast.IfStmt:
		return v.Body
	case *ast.ForStmt:
		return v.Body
	case *ast.RangeStmt:
		return v.Body
	case *ast.SwitchStmt:
		return v.Body
	case *ast.TypeSwitchStmt:
		return v.Body
	case *ast.SelectStmt:
		return v.Body
	}
	return s
}
