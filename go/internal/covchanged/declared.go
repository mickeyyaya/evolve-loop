package covchanged

import (
	"go/ast"
	"go/parser"
	"go/token"
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
