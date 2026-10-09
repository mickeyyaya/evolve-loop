package structure

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func SourceFiles(root string, dirs ...string) ([]string, error) {
	var files []string
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			files = append(files, rel)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func SelectorUses(root, rel, importPath string, members ...string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	name := importName(file, importPath)
	if name == "" {
		return nil, nil
	}
	var uses []string
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && isMemberOf(sel, name, members) {
			uses = append(uses, rel+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+": "+name+"."+sel.Sel.Name)
		}
		return true
	})
	return uses, nil
}

func importName(file *ast.File, importPath string) string {
	for _, imp := range file.Imports {
		if imp.Path.Value != strconv.Quote(importPath) {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return filepath.Base(importPath)
	}
	return ""
}

func isMemberOf(sel *ast.SelectorExpr, pkg string, members []string) bool {
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && slices.Contains(members, sel.Sel.Name)
}
