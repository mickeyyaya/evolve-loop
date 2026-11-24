// Package emptytests refuses a Go test, fuzz or benchmark function with an empty body.
package emptytests

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Offender struct {
	File string
	Line int
	Func string
}

var kinds = []struct{ prefix, param string }{{"Test", "T"}, {"Fuzz", "F"}, {"Benchmark", "B"}}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func Scan(goDir string) ([]Offender, error) {
	fsys := os.DirFS(goDir)
	var found []Offender
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		offenders, err := scanFile(fsys, p)
		found = append(found, offenders...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

func scanFile(fsys fs.FS, p string) ([]Offender, error) {
	src, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	local := testingName(file)
	var found []Offender
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && isTestFunc(fn, local) && len(fn.Body.List) == 0 {
			found = append(found, Offender{File: p, Line: fset.Position(fn.Name.Pos()).Line, Func: fn.Name.Name})
		}
	}
	return found, nil
}

func testingName(file *ast.File) string {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, "\"`") != "testing" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "testing"
	}
	return ""
}

func isTestFunc(fn *ast.FuncDecl, local string) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) > 1 {
		return false
	}
	typ := types.ExprString(params[0].Type)
	for _, k := range kinds {
		if hasTestName(fn.Name.Name, k.prefix) && typ == "*"+local+"."+k.param {
			return true
		}
	}
	return false
}

func hasTestName(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

func Check(found []Offender) error {
	if len(found) == 0 {
		return nil
	}
	sorted := slices.Clone(found)
	slices.SortFunc(sorted, func(a, b Offender) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Func, b.Func))
	})
	lines := make([]string, 0, len(sorted))
	for _, o := range sorted {
		lines = append(lines, fmt.Sprintf("%s:%d: %s has an empty body", o.File, o.Line, o.Func))
	}
	return errors.New(strings.Join(lines, "\n"))
}
