package paralleltest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Serial        map[string]string
	GlobalSetters map[string]bool
}

var processStateCalls = map[string]bool{
	"os.Chdir": true, "os.Setenv": true, "os.Unsetenv": true, "os.Clearenv": true,
}

var processStateVars = map[string]bool{
	"os.Stderr": true, "os.Stdout": true, "os.Stdin": true, "os.Args": true,
}

type parsedFile struct {
	path    string
	pkgName string
	file    *ast.File
}

type pkgIndex struct {
	fset    *token.FileSet
	vars    map[string]bool
	funcs   map[string][]*ast.FuncDecl
	methods map[string][]*ast.FuncDecl
	cfg     Config
}

type testSite struct {
	fd        *ast.FuncDecl
	key       string
	file      string
	qualifier string
}

func Sources(dir string) (map[string]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("glob %s/*.go: %w", dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("glob %s/*.go: no Go files", dir)
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		sources[filepath.Base(path)] = string(raw)
	}
	return sources, nil
}

func Violations(sources map[string]string, cfg Config) []string {
	files, violations := parseSources(sources)
	idx := pkgIndex{fset: files.fset, vars: declaredPackageVars(files), cfg: cfg}
	idx.funcs, idx.methods = packageFuncs(files.parsed)
	seen := map[string]bool{}
	for _, pf := range files.parsed {
		if !strings.HasSuffix(pf.path, "_test.go") {
			continue
		}
		qualifier := strings.TrimSuffix(pf.pkgName, "_test")
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || !isTopLevelTest(fd) {
				continue
			}
			site := testSite{fd: fd, key: pf.pkgName + "." + fd.Name.Name, file: filepath.Base(pf.path), qualifier: qualifier}
			seen[site.key] = true
			violations = append(violations, idx.testViolations(site)...)
		}
	}
	for key := range cfg.Serial {
		if !seen[key] {
			violations = append(violations, key+": listed as serial but no such test exists")
		}
	}
	sort.Strings(violations)
	return violations
}

type parsedSources struct {
	fset   *token.FileSet
	parsed []parsedFile
}

func parseSources(sources map[string]string) (parsedSources, []string) {
	out := parsedSources{fset: token.NewFileSet()}
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var violations []string
	for _, path := range paths {
		file, err := parser.ParseFile(out.fset, path, sources[path], 0)
		if err != nil {
			violations = append(violations, path+": "+err.Error())
			continue
		}
		out.parsed = append(out.parsed, parsedFile{path: path, pkgName: file.Name.Name, file: file})
	}
	return out, violations
}

func (idx pkgIndex) testViolations(site testSite) []string {
	fd, key := site.fd, site.key
	tName := testingTName(fd)
	parallelFirst := startsWithParallel(fd.Body, tName)
	parallelAnywhere := callsAnywhere(fd.Body, tName, "Parallel")
	exempt := callsAnywhere(fd.Body, tName, "Setenv", "Chdir")
	_, listed := idx.cfg.Serial[key]
	var violations []string
	switch {
	case parallelFirst && listed:
		violations = append(violations, key+": calls t.Parallel but is listed as serial")
	case !parallelFirst && parallelAnywhere:
		violations = append(violations, key+" ("+site.file+"): t.Parallel must be the first statement of the test body")
	case !parallelFirst && exempt && listed:
		violations = append(violations, key+": calls t.Setenv or t.Chdir so it needs no serial entry")
	case !parallelFirst && !exempt && !listed:
		violations = append(violations, key+" ("+site.file+"): does not call t.Parallel and is not listed as serial")
	}
	if !parallelAnywhere {
		return violations
	}
	prefix := key + " (" + site.file + "): a parallel test must not change process state"
	for _, stmt := range idx.mutations(fd, tName, site.qualifier) {
		violations = append(violations, prefix+": "+stmt)
	}
	for _, helper := range idx.reachableHelpers(fd) {
		for _, stmt := range idx.mutations(helper, testingTName(helper), site.qualifier) {
			violations = append(violations, prefix+" via helper "+helper.Name.Name+": "+stmt)
		}
	}
	return violations
}

func testingTName(fd *ast.FuncDecl) string {
	if fd.Type.Params == nil {
		return ""
	}
	for _, field := range fd.Type.Params.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "T" {
			continue
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "testing" {
			continue
		}
		if len(field.Names) == 1 && field.Names[0].Name != "_" {
			return field.Names[0].Name
		}
	}
	return ""
}

func isTopLevelTest(fd *ast.FuncDecl) bool {
	return fd.Recv == nil && fd.Body != nil &&
		strings.HasPrefix(fd.Name.Name, "Test") && fd.Name.Name != "TestMain"
}

func isMethodCallOn(n ast.Node, recv, method string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == recv
}

func startsWithParallel(body *ast.BlockStmt, tName string) bool {
	if tName == "" || len(body.List) == 0 {
		return false
	}
	first, ok := body.List[0].(*ast.ExprStmt)
	return ok && isMethodCallOn(first.X, tName, "Parallel")
}

func callsAnywhere(body *ast.BlockStmt, tName string, methods ...string) bool {
	if tName == "" {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		for _, method := range methods {
			if isMethodCallOn(n, tName, method) {
				found = true
			}
		}
		return !found
	})
	return found
}

func declaredPackageVars(files parsedSources) map[string]bool {
	vars := map[string]bool{}
	for _, pf := range files.parsed {
		for _, decl := range pf.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if name.Name != "_" {
						vars[name.Name] = true
					}
				}
			}
		}
	}
	return vars
}

func packageFuncs(files []parsedFile) (funcs, methods map[string][]*ast.FuncDecl) {
	funcs, methods = map[string][]*ast.FuncDecl{}, map[string][]*ast.FuncDecl{}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if fd.Recv == nil {
				funcs[fd.Name.Name] = append(funcs[fd.Name.Name], fd)
			} else {
				methods[fd.Name.Name] = append(methods[fd.Name.Name], fd)
			}
		}
	}
	return funcs, methods
}

func (idx pkgIndex) callees(caller *ast.FuncDecl, call *ast.CallExpr) []*ast.FuncDecl {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return idx.funcs[fn.Name]
	case *ast.SelectorExpr:
		recv, ok := fn.X.(*ast.Ident)
		if ok && idx.vars[recv.Name] && !localNames(caller)[recv.Name] {
			return idx.methods[fn.Sel.Name]
		}
	}
	return nil
}

func (idx pkgIndex) reachableHelpers(root *ast.FuncDecl) []*ast.FuncDecl {
	visited := map[*ast.FuncDecl]bool{root: true}
	queue := []*ast.FuncDecl{root}
	var reached []*ast.FuncDecl
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ast.Inspect(current.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, callee := range idx.callees(current, call) {
				if !visited[callee] {
					visited[callee] = true
					reached = append(reached, callee)
					queue = append(queue, callee)
				}
			}
			return true
		})
	}
	return reached
}

func localNames(fd *ast.FuncDecl) map[string]bool {
	locals := map[string]bool{}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				locals[n.Name] = true
			}
		}
	}
	addFields(fd.Recv)
	addFields(fd.Type.Params)
	addFields(fd.Type.Results)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				for _, l := range v.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						locals[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range v.Names {
				locals[id.Name] = true
			}
		case *ast.RangeStmt:
			if v.Tok == token.DEFINE {
				for _, e := range []ast.Expr{v.Key, v.Value} {
					if id, ok := e.(*ast.Ident); ok {
						locals[id.Name] = true
					}
				}
			}
		case *ast.FuncLit:
			addFields(v.Type.Params)
		}
		return true
	})
	return locals
}

func writtenPackageName(target ast.Expr, qualifier string, locals map[string]bool) string {
	for {
		switch v := target.(type) {
		case *ast.Ident:
			if locals[v.Name] {
				return ""
			}
			return v.Name
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && !locals[id.Name] {
				if processStateVars[id.Name+"."+v.Sel.Name] {
					return id.Name + "." + v.Sel.Name
				}
				if id.Name == qualifier {
					return v.Sel.Name
				}
			}
			target = v.X
		case *ast.IndexExpr:
			target = v.X
		case *ast.StarExpr:
			target = v.X
		case *ast.ParenExpr:
			target = v.X
		default:
			return ""
		}
	}
}

func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

func isStdDescriptorNewFile(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NewFile" || len(call.Args) == 0 {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "os" {
		return false
	}
	arg := call.Args[0]
	if conv, ok := arg.(*ast.CallExpr); ok && len(conv.Args) == 1 {
		arg = conv.Args[0]
	}
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	n, err := strconv.Atoi(lit.Value)
	return err == nil && n >= 0 && n <= 2
}

func (idx pkgIndex) isStateCall(call *ast.CallExpr, tName string) bool {
	if isStdDescriptorNewFile(call) {
		return true
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if idx.cfg.GlobalSetters[sel.Sel.Name] {
		return true
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return processStateCalls[id.Name+"."+sel.Sel.Name] ||
		(id.Name == tName && (sel.Sel.Name == "Setenv" || sel.Sel.Name == "Chdir"))
}

func (idx pkgIndex) mutations(fd *ast.FuncDecl, tName, qualifier string) []string {
	locals := localNames(fd)
	isShared := func(target ast.Expr) bool {
		name := writtenPackageName(target, qualifier, locals)
		return processStateVars[name] || idx.vars[name]
	}
	var found []string
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				return true
			}
			for _, l := range v.Lhs {
				if isShared(l) {
					found = append(found, renderNode(idx.fset, v))
					break
				}
			}
		case *ast.IncDecStmt:
			if isShared(v.X) {
				found = append(found, renderNode(idx.fset, v))
			}
		case *ast.CallExpr:
			if idx.isStateCall(v, tName) {
				found = append(found, renderNode(idx.fset, v))
			}
		}
		return true
	})
	return found
}
