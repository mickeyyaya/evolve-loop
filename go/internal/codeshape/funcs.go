package codeshape

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

const (
	maxLines  = 30
	maxCyclo  = 10
	maxNest   = 3
	maxParams = 4
)

type Func struct {
	File                       string
	Name                       string
	Start, End                 int
	Lines, Cyclo, Nest, Params int
}

func (f Func) Conforms() bool {
	return f.Lines <= maxLines && f.Cyclo <= maxCyclo && f.Nest <= maxNest && f.Params <= maxParams
}

func Conforming(funcs []Func) (k, n int) {
	for _, f := range funcs {
		if f.Conforms() {
			k++
		}
	}
	return k, len(funcs)
}

func Production(rel string) bool {
	if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "acs/") || strings.HasPrefix(rel, "test/") {
		return false
	}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
		if seg == "testdata" || seg == "vendor" || strings.HasPrefix(seg, ".") && seg != "." {
			return false
		}
	}
	return true
}

func parseFile(file string, src []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	return f, fset, err
}

func ParseFuncs(file string, src []byte) ([]Func, error) {
	f, fset, err := parseFile(file, src)
	if err != nil {
		return nil, err
	}
	return funcsOf(file, f, fset), nil
}

func funcsOf(file string, f *ast.File, fset *token.FileSet) []Func {
	var out []Func
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		start, end := fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line
		out = append(out, Func{File: file, Name: funcName(fd), Start: start, End: end, Lines: end - start + 1,
			Cyclo: cyclomatic(fd), Nest: nesting(fd.Body), Params: paramCount(fd.Type.Params)})
	}
	return out
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return typeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func typeName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.StarExpr:
		return typeName(x.X)
	case *ast.IndexExpr:
		return typeName(x.X)
	case *ast.IndexListExpr:
		return typeName(x.X)
	}
	return fmt.Sprint(t)
}

func cyclomatic(n ast.Node) int {
	c := 1
	ast.Inspect(n, func(x ast.Node) bool {
		switch t := x.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			c += min(1, len(t.List))
		case *ast.CommClause:
			if t.Comm != nil {
				c++
			}
		case *ast.BinaryExpr:
			if t.Op == token.LAND || t.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

func nesting(body ast.Node) int {
	n := &nester{}
	n.walk(body, 0)
	return n.deepest
}

type nester struct{ deepest int }

func (n *nester) walk(node ast.Node, depth int) {
	ast.Inspect(node, func(x ast.Node) bool {
		if x == node {
			return true
		}
		switch s := x.(type) {
		case *ast.IfStmt:
			n.ifChain(s, depth)
			return false
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit:
			n.deepest = max(n.deepest, depth+1)
			n.walk(x, depth+1)
			return false
		}
		return true
	})
}

func (n *nester) ifChain(s *ast.IfStmt, depth int) {
	n.deepest = max(n.deepest, depth+1)
	for _, part := range []ast.Node{s.Init, s.Cond, s.Body} {
		if part != nil {
			n.walk(part, depth+1)
		}
	}
	switch e := s.Else.(type) {
	case *ast.IfStmt:
		n.ifChain(e, depth)
	case *ast.BlockStmt:
		n.walk(e, depth+1)
	}
}

func paramCount(fields *ast.FieldList) int {
	n := 0
	for _, f := range fields.List {
		n += max(1, len(f.Names))
	}
	return n
}
