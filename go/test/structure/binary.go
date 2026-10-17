package structure

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

var processImports = []string{"os/exec", "github.com/mickeyyaya/evolve-loop/go/internal/sysexec"}

func ExecImporters(root string, dirs ...string) ([]string, error) {
	return filesWhere(root, dirs, runsAProcess)
}

func runsAProcess(file *ast.File) bool {
	return slices.ContainsFunc(file.Imports, func(imp *ast.ImportSpec) bool {
		return slices.Contains(processImports, strings.Trim(imp.Path.Value, "\"`"))
	})
}

func LiteralFiles(root, needle string, dirs ...string) ([]string, error) {
	return filesWhere(root, dirs, func(file *ast.File) bool { return hasLiteralWith(file, needle) })
}

func filesWhere(root string, dirs []string, keep func(*ast.File) bool) ([]string, error) {
	files, err := SourceFiles(root, dirs...)
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, rel := range files {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		if keep(file) {
			matched = append(matched, rel)
		}
	}
	return matched, nil
}

func hasLiteralWith(file *ast.File, want string) bool {
	return hasLiteral(file, func(value string) bool { return strings.Contains(value, want) })
}

func hasLiteralEqual(file *ast.File, want string) bool {
	return hasLiteral(file, func(value string) bool { return value == want })
}

func hasLiteral(file *ast.File, match func(string) bool) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value, err := strconv.Unquote(lit.Value)
			found = found || (err == nil && match(value))
		}
		return !found
	})
	return found
}
