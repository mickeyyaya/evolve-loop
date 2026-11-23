package docstests

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const evolveLoopModule = "github.com/mickeyyaya/evolve-loop/go"

type Selection struct {
	Package string
	Tests   []string
}

func (s Selection) Regexp() string {
	if len(s.Tests) == 0 {
		return ""
	}
	if len(s.Tests) == 1 {
		return "^" + regexp.QuoteMeta(s.Tests[0]) + "$"
	}
	quoted := make([]string, len(s.Tests))
	for i, name := range s.Tests {
		quoted[i] = regexp.QuoteMeta(name)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

func Format(selections []Selection) string {
	var b strings.Builder
	for _, s := range selections {
		if len(s.Tests) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\t%s\n", s.Package, s.Regexp())
	}
	return b.String()
}

func Select(projectRoot string, changed []string) ([]Selection, error) {
	changedSet := normalizeChanged(changed)
	if len(changedSet) == 0 {
		return nil, nil
	}
	goDir := filepath.Join(projectRoot, "go")
	info, err := os.Stat(goDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", goDir)
	}
	byPkg := map[string]map[string]bool{}
	if isEvolveLoopModule(goDir) {
		addDataFileReads(byPkg, changedSet)
	}
	if err := scanTestTree(byPkg, goDir, changedSet); err != nil {
		return nil, err
	}
	return buildSelections(byPkg), nil
}

func normalizeChanged(changed []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range changed {
		p = strings.TrimPrefix(filepath.ToSlash(p), "./")
		if p != "" {
			out[p] = true
		}
	}
	return out
}

func isEvolveLoopModule(goDir string) bool {
	f, err := os.Open(filepath.Join(goDir, "go.mod"))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`) == evolveLoopModule
		}
	}
	return false
}

func addDataFileReads(byPkg map[string]map[string]bool, changed map[string]bool) {
	for _, row := range repocontract.DataFileReads() {
		if row.Path == repocontract.WholeRepository || strings.HasPrefix(row.Package, "./acs/") {
			continue
		}
		if matchesContractPath(row.Path, changed) {
			addTests(byPkg, row.Package, row.Tests)
		}
	}
}

func matchesContractPath(contractPath string, changed map[string]bool) bool {
	if changed[contractPath] {
		return true
	}
	if !strings.HasSuffix(contractPath, "/") {
		return false
	}
	for p := range changed {
		if strings.HasPrefix(p, contractPath) {
			return true
		}
	}
	return false
}

func addTests(byPkg map[string]map[string]bool, pkg string, tests []string) {
	if len(tests) == 0 {
		return
	}
	set, ok := byPkg[pkg]
	if !ok {
		set = map[string]bool{}
		byPkg[pkg] = set
	}
	for _, name := range tests {
		set[name] = true
	}
}

func buildSelections(byPkg map[string]map[string]bool) []Selection {
	pkgs := make([]string, 0, len(byPkg))
	for pkg, set := range byPkg {
		if len(set) > 0 {
			pkgs = append(pkgs, pkg)
		}
	}
	slices.Sort(pkgs)
	out := make([]Selection, 0, len(pkgs))
	for _, pkg := range pkgs {
		tests := make([]string, 0, len(byPkg[pkg]))
		for name := range byPkg[pkg] {
			tests = append(tests, name)
		}
		slices.Sort(tests)
		out = append(out, Selection{Package: pkg, Tests: tests})
	}
	return out
}

func scanTestTree(byPkg map[string]map[string]bool, goDir string, changed map[string]bool) error {
	dirTests := map[string][]string{}
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipTestDir(path, goDir, d.Name())
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			dir := filepath.Dir(path)
			dirTests[dir] = append(dirTests[dir], path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for dir, paths := range dirTests {
		if err := scanPackageDir(byPkg, goDir, dir, paths, changed); err != nil {
			return err
		}
	}
	return nil
}

func skipTestDir(path, goDir, name string) error {
	if path == goDir {
		return nil
	}
	if name == "testdata" || name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") {
		return filepath.SkipDir
	}
	return nil
}

func scanPackageDir(byPkg map[string]map[string]bool, goDir, dir string, paths []string, changed map[string]bool) error {
	pkgPattern := "."
	if dir != goDir {
		pkgPattern = "./" + strings.TrimPrefix(filepath.ToSlash(dir), filepath.ToSlash(goDir)+"/")
	}
	byName, err := parseUntaggedTests(paths)
	if err != nil {
		return err
	}
	for _, files := range byName {
		addTests(byPkg, pkgPattern, analyzePackage(files, changed))
	}
	return nil
}

func parseUntaggedTests(paths []string) (map[string][]*ast.File, error) {
	fset := token.NewFileSet()
	byName := map[string][]*ast.File{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if hasBuildConstraint(src) {
			continue
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, err
		}
		byName[file.Name.Name] = append(byName[file.Name.Name], file)
	}
	return byName, nil
}

func hasBuildConstraint(src []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) {
			return true
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

func analyzePackage(files []*ast.File, changed map[string]bool) []string {
	decls := collectPackageStringDecls(files)
	matchedIdents := map[string]bool{}
	for name := range decls {
		if val, ok := resolveStringDecl(name, decls, map[string]bool{}); ok && changed[val] {
			matchedIdents[name] = true
		}
	}
	return selectPackageTests(files, changed, matchedIdents)
}

func collectPackageStringDecls(files []*ast.File) map[string]ast.Expr {
	decls := map[string]ast.Expr{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for i, ident := range vs.Names {
						if i < len(vs.Values) && ident.Name != "_" {
							decls[ident.Name] = vs.Values[i]
						}
					}
				}
			}
		}
	}
	return decls
}

func resolveStringDecl(name string, decls map[string]ast.Expr, seen map[string]bool) (string, bool) {
	if seen[name] {
		return "", false
	}
	expr, ok := decls[name]
	if !ok {
		return "", false
	}
	seen[name] = true
	return evalStringExpr(expr, decls, seen)
}

func evalStringExpr(expr ast.Expr, decls map[string]ast.Expr, seen map[string]bool) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.Ident:
		return resolveStringDecl(e.Name, decls, seen)
	case *ast.ParenExpr:
		return evalStringExpr(e.X, decls, seen)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, lok := evalStringExpr(e.X, decls, seen)
		right, rok := evalStringExpr(e.Y, decls, seen)
		if !lok || !rok {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func selectPackageTests(files []*ast.File, changed, matchedIdents map[string]bool) []string {
	direct := map[string]bool{}
	calls := map[string][]string{}
	var tests []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			name := fn.Name.Name
			if isTestFuncName(name) {
				tests = append(tests, name)
			}
			hit, called := scanFuncBody(fn.Body, changed, matchedIdents)
			if hit {
				direct[name] = true
			}
			calls[name] = append(calls[name], called...)
		}
	}
	var out []string
	for _, name := range tests {
		if reachesMatched(name, direct, calls, map[string]bool{}) {
			out = append(out, name)
		}
	}
	return out
}

func scanFuncBody(body *ast.BlockStmt, changed, matchedIdents map[string]bool) (bool, []string) {
	hit := false
	var called []string
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			ast.Inspect(node.X, func(inner ast.Node) bool {
				if scanLeaf(inner, changed, matchedIdents) {
					hit = true
				}
				return true
			})
			return false
		case *ast.CallExpr:
			if id, ok := node.Fun.(*ast.Ident); ok {
				called = append(called, id.Name)
			}
		default:
			if scanLeaf(node, changed, matchedIdents) {
				hit = true
			}
		}
		return true
	})
	return hit, called
}

func scanLeaf(node ast.Node, changed, matchedIdents map[string]bool) bool {
	switch x := node.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			if s, err := strconv.Unquote(x.Value); err == nil && changed[s] {
				return true
			}
		}
	case *ast.Ident:
		if matchedIdents[x.Name] {
			return true
		}
	}
	return false
}

func reachesMatched(name string, direct map[string]bool, calls map[string][]string, seen map[string]bool) bool {
	if direct[name] {
		return true
	}
	if seen[name] {
		return false
	}
	seen[name] = true
	for _, next := range calls[name] {
		if reachesMatched(next, direct, calls, seen) {
			return true
		}
	}
	return false
}

func isTestFuncName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) == len("Test") {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}
