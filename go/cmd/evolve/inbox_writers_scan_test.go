package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type scannedFunc struct {
	decl    *ast.FuncDecl
	params  []string
	imports map[string]bool
}

type scannedPackage struct {
	funcs        map[string][]scannedFunc
	inboxNames   map[string]bool
	returnsInbox map[string]bool
	writerHelper map[string]bool
}

func inboxWriteSites(t *testing.T, moduleDir string) []string {
	t.Helper()
	packages := scanProductionPackages(t, moduleDir)
	var sites []string
	for dir, pkg := range packages {
		pkg.resolveHelpers()
		for name, fns := range pkg.funcs {
			for _, fn := range fns {
				if pkg.writesInbox(fn, pkg.taint(fn, nil)) {
					sites = append(sites, dir+"."+name)
				}
			}
		}
	}
	slices.Sort(sites)
	return slices.Compact(sites)
}

func scanProductionPackages(t *testing.T, moduleDir string) map[string]*scannedPackage {
	t.Helper()
	packages := map[string]*scannedPackage{}
	err := filepath.WalkDir(moduleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipNonProductionDir(path, moduleDir, d.Name())
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return addScannedFile(packages, moduleDir, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	return packages
}

func skipNonProductionDir(path, moduleDir, name string) error {
	if path != moduleDir && (name == "testdata" || name == "vendor" || name == "acs" || strings.HasPrefix(name, ".")) {
		return filepath.SkipDir
	}
	return nil
}

func addScannedFile(packages map[string]*scannedPackage, moduleDir, path string) error {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(moduleDir, filepath.Dir(path))
	if err != nil {
		return err
	}
	dir := filepath.ToSlash(rel)
	if packages[dir] == nil {
		packages[dir] = &scannedPackage{funcs: map[string][]scannedFunc{}, inboxNames: map[string]bool{}, returnsInbox: map[string]bool{}, writerHelper: map[string]bool{}}
	}
	imports := importNames(file)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				packages[dir].funcs[d.Name.Name] = append(packages[dir].funcs[d.Name.Name], scannedFunc{decl: d, params: paramNames(d), imports: imports})
			}
		case *ast.GenDecl:
			packages[dir].addInboxNames(d)
		}
	}
	return nil
}

func (p *scannedPackage) addInboxNames(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			if i < len(vs.Values) && p.mentionsInbox(scannedFunc{}, vs.Values[i], nil) {
				p.inboxNames[name.Name] = true
			}
		}
	}
}

func importNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}
	return names
}

func paramNames(fd *ast.FuncDecl) []string {
	var names []string
	for _, field := range fd.Type.Params.List {
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

func (p *scannedPackage) resolveHelpers() {
	for changed := true; changed; {
		changed = false
		for name, fns := range p.funcs {
			for _, fn := range fns {
				if !p.returnsInbox[name] && p.returnsTainted(fn) {
					p.returnsInbox[name], changed = true, true
				}
				if !p.writerHelper[name] && len(fn.params) > 0 && p.writesInbox(fn, p.taint(fn, fn.params)) && !p.writesInbox(fn, p.taint(fn, nil)) {
					p.writerHelper[name], changed = true, true
				}
			}
		}
	}
}

func (p *scannedPackage) taint(fn scannedFunc, seeded []string) map[string]bool {
	tainted := map[string]bool{}
	for _, name := range seeded {
		tainted[name] = true
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
			for _, f := range flowsOf(n) {
				if key := taintKey(f.target); key != "" && !tainted[key] && p.mentionsInbox(fn, f.source, tainted) {
					tainted[key], changed = true, true
				}
			}
			return true
		})
	}
	return tainted
}

type valueFlow struct{ target, source ast.Expr }

func flowsOf(n ast.Node) []valueFlow {
	switch v := n.(type) {
	case *ast.AssignStmt:
		flows := make([]valueFlow, 0, len(v.Lhs))
		for i, lhs := range v.Lhs {
			source := v.Rhs[0]
			if len(v.Rhs) == len(v.Lhs) {
				source = v.Rhs[i]
			}
			flows = append(flows, valueFlow{lhs, source})
		}
		return flows
	case *ast.ValueSpec:
		var flows []valueFlow
		for i, name := range v.Names {
			if i < len(v.Values) {
				flows = append(flows, valueFlow{name, v.Values[i]})
			}
		}
		return flows
	case *ast.RangeStmt:
		return []valueFlow{{v.Key, v.X}, {v.Value, v.X}}
	case *ast.CallExpr:
		return walkCallbackFlows(v)
	}
	return nil
}

func walkCallbackFlows(call *ast.CallExpr) []valueFlow {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 || (sel.Sel.Name != "Walk" && sel.Sel.Name != "WalkDir") {
		return nil
	}
	if pkg, isIdent := sel.X.(*ast.Ident); !isIdent || pkg.Name != "filepath" {
		return nil
	}
	lit, ok := call.Args[1].(*ast.FuncLit)
	if !ok || len(lit.Type.Params.List) == 0 || len(lit.Type.Params.List[0].Names) == 0 {
		return nil
	}
	return []valueFlow{{lit.Type.Params.List[0].Names[0], call.Args[0]}}
}

func taintKey(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		if v.Name != "_" {
			return v.Name
		}
	case *ast.SelectorExpr:
		if x := taintKey(v.X); x != "" {
			return x + "." + v.Sel.Name
		}
	}
	return ""
}

func (p *scannedPackage) returnsTainted(fn scannedFunc) bool {
	tainted := p.taint(fn, nil)
	found := false
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, e := range ret.Results {
				found = found || p.mentionsInbox(fn, e, tainted)
			}
		}
		return true
	})
	return found
}

func (p *scannedPackage) writesInbox(fn scannedFunc, tainted map[string]bool) bool {
	found := false
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		if target, isWrite := writeTarget(call); isWrite {
			found = p.mentionsInbox(fn, target, tainted)
		}
		if name := localCallee(fn, call); p.writerHelper[name] {
			for _, arg := range call.Args {
				found = found || p.mentionsInbox(fn, arg, tainted)
			}
		}
		return !found
	})
	return found
}

func writeTarget(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return nil, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	switch {
	case !ok:
		return nil, false
	case pkg.Name == "atomicwrite":
		return call.Args[0], true
	case pkg.Name == "os" && slices.Contains([]string{"WriteFile", "Create", "OpenFile", "CreateTemp"}, sel.Sel.Name):
		return call.Args[0], true
	case pkg.Name == "os" && slices.Contains([]string{"Rename", "Link"}, sel.Sel.Name) && len(call.Args) == 2:
		return call.Args[1], true
	}
	return nil, false
}

func localCallee(fn scannedFunc, call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if id, isIdent := f.X.(*ast.Ident); isIdent && fn.imports[id.Name] {
			return ""
		}
		return f.Sel.Name
	}
	return ""
}

func (p *scannedPackage) mentionsInbox(fn scannedFunc, e ast.Expr, tainted map[string]bool) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return isInboxPathLiteral(v)
	case *ast.Ident:
		return tainted[v.Name] || p.inboxNames[v.Name] || (!fn.imports[v.Name] && isInboxName(v.Name))
	case *ast.SelectorExpr:
		return tainted[taintKey(v)] || isInboxName(v.Sel.Name) || p.mentionsInbox(fn, v.X, tainted)
	case *ast.IndexExpr:
		return p.mentionsInbox(fn, v.X, tainted)
	case *ast.CompositeLit:
		return p.anyElementMentionsInbox(fn, v.Elts, tainted)
	case *ast.KeyValueExpr:
		return p.mentionsInbox(fn, v.Value, tainted)
	case *ast.BinaryExpr:
		return p.mentionsInbox(fn, v.X, tainted) || p.mentionsInbox(fn, v.Y, tainted)
	case *ast.ParenExpr:
		return p.mentionsInbox(fn, v.X, tainted)
	case *ast.StarExpr:
		return p.mentionsInbox(fn, v.X, tainted)
	case *ast.CallExpr:
		return p.callBuildsAnInboxPath(fn, v, tainted)
	}
	return false
}

func (p *scannedPackage) callBuildsAnInboxPath(fn scannedFunc, call *ast.CallExpr, tainted map[string]bool) bool {
	if p.returnsInbox[localCallee(fn, call)] {
		return true
	}
	if !buildsAPath(call) {
		return false
	}
	for _, arg := range call.Args {
		if p.mentionsInbox(fn, arg, tainted) {
			return true
		}
	}
	return false
}

func (p *scannedPackage) anyElementMentionsInbox(fn scannedFunc, elts []ast.Expr, tainted map[string]bool) bool {
	for _, e := range elts {
		if p.mentionsInbox(fn, e, tainted) {
			return true
		}
	}
	return false
}

func buildsAPath(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "filepath" || pkg.Name == "path" || pkg.Name == "strings" || (pkg.Name == "fmt" && sel.Sel.Name == "Sprintf"))
}

func isInboxPathLiteral(lit *ast.BasicLit) bool {
	if lit.Kind != token.STRING {
		return false
	}
	v, err := strconv.Unquote(lit.Value)
	return err == nil && (v == "inbox" || strings.HasPrefix(v, "inbox/") || strings.Contains(v, ".evolve/inbox"))
}

var inboxNamePattern = regexp.MustCompile(`(?i)(^|[a-z0-9])inbox(dir|path|root)?$`)

func isInboxName(name string) bool {
	return inboxNamePattern.MatchString(name)
}
