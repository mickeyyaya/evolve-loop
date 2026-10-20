package covchanged

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"io"
	"path"
	"slices"
	"sort"
	"strings"
)

func Measure(src Sources) (Report, error) {
	var report Report
	for _, rel := range goPaths(src.Changed) {
		funcs, reason, err := measureFile(src, rel)
		if err != nil {
			return Report{}, err
		}
		if reason != "" {
			report.Exempt = append(report.Exempt, Exemption{File: rel, Reason: reason})
		}
		report.Funcs = append(report.Funcs, funcs...)
	}
	return report, nil
}

func goPaths(changed Diff) []string {
	var out []string
	for _, set := range []AddedLines{changed.Added, changed.Removed} {
		for rel := range set {
			if strings.HasSuffix(rel, ".go") && !slices.Contains(out, rel) {
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

func pathExemption(rel string) string {
	dirs := strings.Split(path.Dir(rel), "/")
	switch {
	case strings.HasSuffix(rel, "_test.go"):
		return ReasonTestFile
	case slices.Contains(dirs, "testdata"):
		return ReasonTestdata
	case slices.Contains(dirs, "vendor"):
		return ReasonVendor
	}
	return ""
}

func measureFile(src Sources, rel string) ([]Func, string, error) {
	if reason := pathExemption(rel); reason != "" {
		return nil, reason, nil
	}
	if !src.Packages.Holds(rel) {
		return nil, ReasonOutsidePackages, nil
	}
	data, err := src.ReadFile(rel)
	if err != nil {
		return nil, "", fmt.Errorf("covchanged: read %s: %w", rel, err)
	}
	builds, err := buildsFor(src.Build, rel, data)
	if err != nil {
		return nil, "", fmt.Errorf("covchanged: build constraints of %s: %w", rel, err)
	}
	if !builds {
		return nil, ReasonBuildConstraints, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, data, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, "", fmt.Errorf("covchanged: parse %s: %w", rel, err)
	}
	touched := changes{lines: changedCodeLines(src.Changed.Added[rel], codeLines(data)), gaps: src.Changed.Removed[rel]}
	return funcsTouched(fset, file, rel, touched, src.Profile[rel]), "", nil
}

func buildsFor(ctx build.Context, rel string, data []byte) (bool, error) {
	ctx.JoinPath = path.Join
	ctx.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	return ctx.MatchFile(path.Dir(rel), path.Base(rel))
}

func codeLines(data []byte) map[int]bool {
	file := token.NewFileSet().AddFile("", -1, len(data))
	var s scanner.Scanner
	s.Init(file, data, nil, 0)
	lines := map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return lines
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		for l := file.Line(pos); l <= file.Line(pos+token.Pos(len(lit))); l++ {
			lines[l] = true
		}
	}
}

func changedCodeLines(added []int, code map[int]bool) []int {
	var out []int
	for _, l := range added {
		if code[l] {
			out = append(out, l)
		}
	}
	return out
}

func funcsTouched(fset *token.FileSet, file *ast.File, rel string, touched changes, blocks []Block) []Func {
	var out []Func
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		start, end := fset.Position(fd.Pos()), fset.Position(fd.End())
		if !touched.inside(start.Line, end.Line) {
			continue
		}
		name := funcName(fd)
		f := Func{File: rel, Line: start.Line, Name: name, Key: path.Dir(rel) + "." + name}
		out = append(out, f.count(blocks, start, end))
	}
	return out
}

type changes struct {
	lines []int
	gaps  []int
}

func (c changes) inside(first, last int) bool {
	return slices.ContainsFunc(c.lines, func(l int) bool { return l >= first && l <= last }) ||
		slices.ContainsFunc(c.gaps, func(g int) bool { return g >= first && g < last })
}

func (f Func) count(blocks []Block, start, end token.Position) Func {
	from, to := []int{start.Line, start.Column}, []int{end.Line, end.Column}
	for _, b := range blocks {
		at := []int{b.StartLine, b.StartCol}
		if slices.Compare(at, from) < 0 || slices.Compare(at, to) >= 0 {
			continue
		}
		f.Profiled = true
		f.Statements += b.Statements
		if b.Hit {
			f.Covered += b.Statements
		}
	}
	return f
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return receiverType(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func receiverType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverType(e.X)
	case *ast.ParenExpr:
		return receiverType(e.X)
	case *ast.IndexExpr:
		return receiverType(e.X)
	case *ast.IndexListExpr:
		return receiverType(e.X)
	}
	return types.ExprString(expr)
}
