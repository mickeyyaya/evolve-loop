package acspathscan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Need struct {
	Path string `json:"path"`
	Test string `json:"test"`
	File string `json:"file"`
}

var (
	wholePathRE = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9_./-]*$`)
	segmentRE   = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9_.-]*$`)
	osReaders   = []string{"ReadFile", "Stat", "Lstat", "Open", "OpenFile"}
	acsReaders  = []string{"FileExists", "FileContains", "FileNotContains", "FileMatchesRegex", "JSONFieldEquals", "FileContainsAny", "FileContainsAnyChecked", "CountOccurrencesAny", "CountOccurrencesAnyChecked", "LineContainsAll", "LineContainsAllChecked", "CountInGoFunc"}
)

const repoRootName = "RepoRoot"

func Needs(dir string, member func(string) bool) ([]Need, error) {
	if member == nil {
		return nil, nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("acspathscan: list %s: %w", dir, err)
	}
	var needs []Need
	var errs []error
	for _, file := range files {
		parsed, perr := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if perr != nil {
			errs = append(errs, fmt.Errorf("acspathscan: parse %s: %w", file, perr))
			continue
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			for _, p := range readPaths(fn.Body) {
				if member(p) && !slices.ContainsFunc(needs, func(n Need) bool { return n.Path == p && n.Test == fn.Name.Name }) {
					needs = append(needs, Need{Path: p, Test: fn.Name.Name, File: filepath.Base(file)})
				}
			}
		}
	}
	return needs, errors.Join(errs...)
}

func Snapshot(src, dst string) error {
	files, err := filepath.Glob(filepath.Join(src, "*.go"))
	if err != nil {
		return fmt.Errorf("acspathscan: list %s: %w", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("acspathscan: %w", err)
	}
	for _, file := range files {
		body, rerr := os.ReadFile(file)
		if rerr != nil {
			return fmt.Errorf("acspathscan: %w", rerr)
		}
		if werr := os.WriteFile(filepath.Join(dst, filepath.Base(file)), body, 0o644); werr != nil {
			return fmt.Errorf("acspathscan: %w", werr)
		}
	}
	return nil
}

type scope struct {
	roots map[string]bool
	paths map[string]string
}

func readPaths(body *ast.BlockStmt) []string {
	sc := scope{roots: map[string]bool{}, paths: map[string]string{}}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			sc.bind(identNames(node.Lhs), node.Rhs)
		case *ast.ValueSpec:
			sc.bind(node.Names, node.Values)
		}
		return true
	})
	var out []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isReader(call) {
			return true
		}
		for _, arg := range call.Args {
			if p, found := sc.pathOf(arg); found {
				out = append(out, p)
			}
		}
		return true
	})
	return out
}

func identNames(exprs []ast.Expr) []*ast.Ident {
	var out []*ast.Ident
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok {
			out = append(out, id)
		}
	}
	return out
}

func (sc scope) bind(names []*ast.Ident, values []ast.Expr) {
	if len(names) != len(values) {
		return
	}
	for i, name := range names {
		if isRepoRoot(values[i]) {
			sc.roots[name.Name] = true
		} else if p, ok := sc.pathOf(values[i]); ok {
			sc.paths[name.Name] = p
		}
	}
}

func (sc scope) pathOf(e ast.Expr) (string, bool) {
	switch node := e.(type) {
	case *ast.Ident:
		p, ok := sc.paths[node.Name]
		return p, ok
	case *ast.BasicLit:
		return wholePath(node)
	case *ast.CallExpr:
		return sc.joined(node)
	}
	return "", false
}

func (sc scope) joined(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" || len(call.Args) < 2 {
		return "", false
	}
	rest := call.Args
	if !isLiteral(rest[0]) {
		if !sc.isRoot(rest[0]) {
			return "", false
		}
		rest = rest[1:]
	}
	parts := make([]string, 0, len(rest))
	for _, arg := range rest {
		lit, isLit := arg.(*ast.BasicLit)
		value, isString := "", false
		if isLit {
			value, isString = stringValue(lit)
		}
		if !isString || !segmentRE.MatchString(value) {
			return "", false
		}
		parts = append(parts, value)
	}
	return path.Join(parts...), true
}

func (sc scope) isRoot(e ast.Expr) bool {
	if id, ok := e.(*ast.Ident); ok {
		return sc.roots[id.Name]
	}
	return isRepoRoot(e)
}

func isRepoRoot(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == repoRootName
	case *ast.SelectorExpr:
		return fun.Sel.Name == repoRootName
	}
	return false
}

func isLiteral(e ast.Expr) bool {
	_, ok := e.(*ast.BasicLit)
	return ok
}

func isReader(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch pkg.Name {
	case "os":
		return slices.Contains(osReaders, sel.Sel.Name)
	case "acsassert":
		return slices.Contains(acsReaders, sel.Sel.Name)
	}
	return false
}

func stringValue(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func wholePath(lit *ast.BasicLit) (string, bool) {
	value, isString := stringValue(lit)
	if !isString || !wholePathRE.MatchString(value) || !strings.ContainsAny(value, "./") {
		return "", false
	}
	return value, true
}
