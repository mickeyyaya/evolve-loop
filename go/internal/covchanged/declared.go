package covchanged

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
)

func DeclaredFuncs(file string, data []byte) (map[string]bool, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, decl := range parsed.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
			names[funcName(fd)] = true
		}
	}
	return names, nil
}

func ChangedFuncNames(file string, data []byte, lines []int) ([]string, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, decl := range parsed.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		first, last := fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line
		if slices.ContainsFunc(lines, func(l int) bool { return l >= first && l <= last }) {
			names = append(names, funcName(fd))
		}
	}
	return names, nil
}
