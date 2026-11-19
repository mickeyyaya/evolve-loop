package testaudit

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Finding struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Rule    string `json:"rule"`
}

type srcFile struct {
	name string
	ast  *ast.File
}

type pkgInfo struct {
	dir       string
	files     []srcFile
	strConsts map[string]string
	intConsts map[string]bool
	flagVars  map[string]bool
}

func cleanPath(p string) string { return filepath.Clean(p) }

func Analyze(root string, cfg Config) ([]Finding, error) {
	return analyze(root, cfg, true)
}

func AnalyzeTree(root string, cfg Config) ([]Finding, error) {
	base := cleanPath(root)
	return analyzeWithBase(base, base, cfg, true)
}

func analyze(root string, cfg Config, recursive bool) ([]Finding, error) {
	return analyzeWithBase("", cleanPath(root), cfg, recursive)
}

func analyzeWithBase(base, root string, cfg Config, recursive bool) ([]Finding, error) {
	dirs, err := testDirs(root, recursive)
	if err != nil {
		return nil, err
	}
	var found []Finding
	for _, dir := range dirs {
		pkg, err := loadPackage(dir)
		if err != nil {
			return nil, err
		}
		if base != "" {
			pkg.dir = relSlash(base, dir)
		}
		found = append(found, auditPackage(pkg, cfg)...)
	}
	rows, err := fakeRowFindingsWithBase(base, root, recursive, cfg)
	if err != nil {
		return nil, err
	}
	found = append(found, rows...)
	return finish(found, cfg), nil
}

func relSlash(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return cleanPath(target)
	}
	return filepath.ToSlash(rel)
}

func finish(found []Finding, cfg Config) []Finding {
	seen := map[Finding]bool{}
	var out []Finding
	for _, f := range found {
		if seen[f] || cfg.guarded(f) {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		return a.Rule < b.Rule
	})
	return out
}

func testDirs(root string, recursive bool) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (!recursive || skippedDir(d.Name())) {
			return filepath.SkipDir
		}
		if hasTestFile(path) {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs, err
}

func skippedDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func hasTestFile(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	return len(matches) > 0
}

func loadPackage(dir string) (*pkgInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkg := &pkgInfo{dir: cleanPath(dir), strConsts: map[string]string{}, intConsts: map[string]bool{}, flagVars: map[string]bool{}}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		pkg.files = append(pkg.files, srcFile{name: e.Name(), ast: file})
		pkg.collect(file)
	}
	return pkg, nil
}

func (p *pkgInfo) collect(file *ast.File) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			p.collectSpec(gd.Tok, vs)
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if qualified(call) == "flag.BoolVar" && len(call.Args) > 0 {
			if u, ok := call.Args[0].(*ast.UnaryExpr); ok {
				if id, ok := u.X.(*ast.Ident); ok {
					p.flagVars[id.Name] = true
				}
			}
		}
		return true
	})
}

func (p *pkgInfo) collectSpec(tok token.Token, vs *ast.ValueSpec) {
	if len(vs.Names) != len(vs.Values) {
		return
	}
	for i, name := range vs.Names {
		switch v := vs.Values[i].(type) {
		case *ast.BasicLit:
			if tok == token.CONST {
				recordConst(v, name.Name, p.strConsts, p.intConsts)
			}
		case *ast.CallExpr:
			if tok == token.VAR && strings.HasPrefix(qualified(v), "flag.") {
				p.flagVars[name.Name] = true
			}
		}
	}
}

func recordConst(lit *ast.BasicLit, name string, strs map[string]string, ints map[string]bool) {
	switch lit.Kind {
	case token.STRING:
		if s, err := strconv.Unquote(lit.Value); err == nil {
			strs[name] = s
		}
	case token.INT:
		ints[name] = true
	}
}

func qualified(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name + "." + fn.Sel.Name
		}
	case *ast.Ident:
		return fn.Name
	}
	return ""
}

func auditPackage(pkg *pkgInfo, cfg Config) []Finding {
	var found []Finding
	for _, file := range pkg.files {
		if !strings.HasSuffix(file.name, "_test.go") {
			continue
		}
		for _, decl := range file.ast.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			param, ok := testParam(fd)
			if !ok {
				continue
			}
			ctx := newTestCtx(pkg, cfg, fd, param)
			for _, r := range rules {
				if r.check(ctx) {
					found = append(found, Finding{Package: pkg.dir, Test: fd.Name.Name, Rule: r.id})
				}
			}
		}
	}
	return found
}

func testParam(fd *ast.FuncDecl) (string, bool) {
	if fd.Recv != nil || fd.Body == nil || !strings.HasPrefix(fd.Name.Name, "Test") {
		return "", false
	}
	params := fd.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 || !isTestingType(params[0].Type, "T") {
		return "", false
	}
	return params[0].Names[0].Name, true
}

func isTestingType(e ast.Expr, names ...string) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "testing" {
		return false
	}
	for _, n := range names {
		if sel.Sel.Name == n {
			return true
		}
	}
	return false
}

func fakeRowFindingsWithBase(base, root string, recursive bool, cfg Config) ([]Finding, error) {
	var found []Finding
	for _, row := range cfg.Fakes {
		if row.SuiteDir == "" {
			continue
		}
		suiteDir := cleanPath(row.SuiteDir)
		target := suiteDir
		if base != "" && !filepath.IsAbs(suiteDir) {
			target = filepath.Join(base, filepath.FromSlash(suiteDir))
		}
		if !within(root, target, recursive) {
			continue
		}
		has, err := suiteDeclared(target, row.Tool)
		if err != nil {
			return nil, err
		}
		if !has {
			found = append(found, Finding{Package: suiteDir, Test: "fakes:" + row.Tool, Rule: ruleUnverifiedFake})
		}
	}
	return found, nil
}

func within(root, dir string, recursive bool) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	if !recursive {
		return rel == "."
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func suiteDeclared(dir, tool string) (bool, error) {
	pkg, err := loadPackage(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return suiteDeclaredPkg(pkg, tool), nil
}

func suiteDeclaredPkg(pkg *pkgInfo, tool string) bool {
	ctx := &testCtx{pkg: pkg}
	has := false
	for _, file := range pkg.files {
		ast.Inspect(file.ast, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isSuiteType(lit.Type) {
				return true
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Tool" {
					if s, ok := ctx.constString(kv.Value); ok && s == tool {
						has = true
					}
				}
			}
			return true
		})
	}
	return has
}

func isSuiteType(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Suite" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "fakecontract"
}
