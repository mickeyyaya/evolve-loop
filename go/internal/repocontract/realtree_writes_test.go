package repocontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func TestNoTestWritesIntoTheRealTree(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	fset, byPackage, err := parseTestPackages(root, files)
	if err != nil {
		t.Fatal(err)
	}
	var writers []string
	rootHelpers := 0
	for _, key := range slices.Sorted(maps.Keys(byPackage)) {
		scan := newTreeWriteScan(byPackage[key])
		rootHelpers += scan.rootHelpers()
		for _, finding := range scan.findings(fset) {
			writers = append(writers, relativeTo(root, finding))
		}
	}
	if rootHelpers == 0 {
		t.Fatal("the scan found no test helper that returns the real tree, though many tests locate it; the scan has gone blind, so its clean result proves nothing")
	}
	if len(writers) > 0 {
		t.Errorf("%d test writes land in the real tree. A lane runs its tests in its own worktree while the build floor reads that worktree, so each write is a material path nobody explained; write under t.TempDir() or a gittest fixture, copying the real tracked files in when the real set matters. A name that ever holds the real root is treated as the root everywhere in its function; rename the temp-dir variable:\n  %s", len(writers), strings.Join(writers, "\n  "))
	}
}

func relativeTo(root, finding string) string {
	if rel, err := filepath.Rel(root, finding); err == nil {
		return filepath.ToSlash(rel)
	}
	return finding
}

type rooting struct {
	tree   bool
	params uint64
}

func (r rooting) or(o rooting) rooting {
	return rooting{tree: r.tree || o.tree, params: r.params | o.params}
}

type treeWriteScan struct {
	decls   []*ast.FuncDecl
	funcs   map[string]*ast.FuncDecl
	files   []*ast.File
	globals map[string]rooting
	returns map[string][]rooting
	writes  map[string]uint64
	grew    bool
}

type funcFlow struct {
	scan     *treeWriteScan
	env      map[string]rooting
	shadowed map[*ast.Ident]bool
	movedTo  *ast.CallExpr
}

var osPathWrites = map[string][]int{
	"WriteFile": {0}, "MkdirAll": {0}, "Mkdir": {0}, "Create": {0}, "Rename": {0, 1},
	"Remove": {0}, "RemoveAll": {0}, "Symlink": {1}, "Link": {1}, "Chmod": {0},
	"Chtimes": {0}, "Truncate": {0}, "CopyFS": {0}, "MkdirTemp": {0}, "CreateTemp": {0},
	"TempDir": {0}, "TempFile": {0},
}

var openWriteFlags = []string{"O_CREATE", "O_WRONLY", "O_RDWR", "O_APPEND", "O_TRUNC"}

var passPathThrough = []string{
	"filepath.Abs", "filepath.Clean", "filepath.Dir", "filepath.EvalSymlinks", "filepath.FromSlash", "filepath.ToSlash",
	"path.Clean", "path.Dir", "strings.TrimSpace", "strings.TrimSuffix", "strings.TrimRight", "bytes.TrimSpace",
}

func parseTestPackages(root string, files []string) (*token.FileSet, map[string][]*ast.File, error) {
	fset := token.NewFileSet()
	byPackage := map[string][]*ast.File{}
	for _, rel := range files {
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		key := packageKey(path.Dir(rel), file.Name.Name)
		byPackage[key] = append(byPackage[key], file)
	}
	return fset, byPackage, nil
}

func packageKey(dir, name string) string {
	return dir + " " + name
}

func newTreeWriteScan(files []*ast.File) *treeWriteScan {
	s := &treeWriteScan{files: files, funcs: map[string]*ast.FuncDecl{}, globals: map[string]rooting{}, returns: map[string][]rooting{}, writes: map[string]uint64{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				s.decls = append(s.decls, fn)
				if fn.Recv == nil {
					s.funcs[fn.Name.Name] = fn
				}
			}
		}
	}
	for s.grew = true; s.grew; {
		s.grew = false
		s.pass(nil)
	}
	return s
}

func (s *treeWriteScan) returnsTheRealTree(name string) bool {
	return len(s.returns[name]) > 0 && s.returns[name][0].tree
}

func (s *treeWriteScan) rootHelpers() int {
	n := 0
	for name := range s.returns {
		if s.returnsTheRealTree(name) {
			n++
		}
	}
	return n
}

func (s *treeWriteScan) findings(fset *token.FileSet) []string {
	var found []string
	s.pass(func(call *ast.CallExpr) {
		found = append(found, fmt.Sprintf("%s: %s", fset.Position(call.Pos()), callName(call)))
	})
	return found
}

func callName(call *ast.CallExpr) string {
	if q := qualified(call); q != "" {
		return q
	}
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return "call"
}

func (s *treeWriteScan) pass(report func(*ast.CallExpr)) {
	top := &funcFlow{scan: s, env: s.globals}
	for _, file := range s.files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && top.bindDecl(gen) {
				s.grew = true
			}
		}
	}
	for _, fn := range s.decls {
		s.analyze(fn, report)
	}
}

func (s *treeWriteScan) analyze(fn *ast.FuncDecl, report func(*ast.CallExpr)) {
	f := &funcFlow{scan: s, env: map[string]rooting{}, shadowed: map[*ast.Ident]bool{}, movedTo: firstWorkingDirectoryMove(fn.Body)}
	markShadowed(fn.Body, fn.Recv, fn.Type, nil, f.shadowed)
	for i, param := range paramNames(fn.Type) {
		f.env[param] = rooting{params: 1 << min(i, 63)}
	}
	for f.bind(fn.Body) {
	}
	writes := f.sinks(fn.Body, report)
	if fn.Recv != nil {
		return
	}
	var results []rooting
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			results = mergeResults(results, f.resultsOf(ret.Results))
		}
		_, closure := n.(*ast.FuncLit)
		return !closure
	})
	s.record(fn.Name.Name, results, writes)
}

func (s *treeWriteScan) record(name string, results []rooting, writes uint64) {
	merged := mergeResults(s.returns[name], results)
	if !slices.Equal(merged, s.returns[name]) || s.writes[name]|writes != s.writes[name] {
		s.grew = true
	}
	s.returns[name] = merged
	s.writes[name] |= writes
}

func mergeResults(a, b []rooting) []rooting {
	out := slices.Clone(a)
	for i, r := range b {
		if i < len(out) {
			out[i] = out[i].or(r)
		} else {
			out = append(out, r)
		}
	}
	return out
}

func paramNames(ft *ast.FuncType) []string {
	var names []string
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "_")
		}
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

func markShadowed(body *ast.BlockStmt, recv *ast.FieldList, ft *ast.FuncType, outer map[string]bool, shadowed map[*ast.Ident]bool) {
	scope := ownDeclarations(body, recv, ft)
	for name := range outer {
		scope[name] = true
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			markShadowed(n.Body, nil, n.Type, scope, shadowed)
			return false
		case *ast.Ident:
			shadowed[n] = scope[n.Name]
		}
		return true
	})
}

func ownDeclarations(body *ast.BlockStmt, recv *ast.FieldList, ft *ast.FuncType) map[string]bool {
	declared := map[string]bool{}
	declare := func(idents ...*ast.Ident) {
		for _, id := range idents {
			declared[id.Name] = true
		}
	}
	for _, fields := range []*ast.FieldList{recv, ft.Params, ft.Results} {
		if fields != nil {
			for _, field := range fields.List {
				declare(field.Names...)
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				declare(identsOf(n.Lhs...)...)
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				declare(identsOf(n.Key, n.Value)...)
			}
		case *ast.ValueSpec:
			declare(n.Names...)
		}
		return true
	})
	return declared
}

func identsOf(exprs ...ast.Expr) []*ast.Ident {
	var idents []*ast.Ident
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok {
			idents = append(idents, id)
		}
	}
	return idents
}

func firstWorkingDirectoryMove(body *ast.BlockStmt) *ast.CallExpr {
	var first *ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit, *ast.DeferStmt:
			return false
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Chdir" && len(n.Args) == 1 && (first == nil || n.Pos() < first.Pos()) {
				first = n
			}
		}
		return true
	})
	return first
}

func (f *funcFlow) workingDirectoryAt(getwd *ast.CallExpr) rooting {
	if f.movedTo == nil || getwd.Pos() < f.movedTo.End() {
		return rooting{tree: true}
	}
	return f.of(f.movedTo.Args[0])
}

func isVariadic(ft *ast.FuncType) bool {
	params := ft.Params.List
	if len(params) == 0 {
		return false
	}
	_, ok := params[len(params)-1].Type.(*ast.Ellipsis)
	return ok
}

func (f *funcFlow) bind(body ast.Node) (grew bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			grew = f.assign(n.Lhs, n.Rhs) || grew
		case *ast.GenDecl:
			grew = f.bindDecl(n) || grew
		case *ast.RangeStmt:
			if n.Value != nil {
				grew = f.assign([]ast.Expr{n.Value}, []ast.Expr{n.X}) || grew
			}
		}
		return true
	})
	return grew
}

func (f *funcFlow) bindDecl(gen *ast.GenDecl) (grew bool) {
	for _, spec := range gen.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
			lhs := make([]ast.Expr, len(vs.Names))
			for i, n := range vs.Names {
				lhs[i] = n
			}
			grew = f.assign(lhs, vs.Values) || grew
		}
	}
	return grew
}

func (f *funcFlow) assign(lhs, rhs []ast.Expr) (grew bool) {
	values := f.resultsOf(rhs)
	for i, target := range lhs {
		id, ok := target.(*ast.Ident)
		if !ok || i >= len(values) || id.Name == "_" {
			continue
		}
		if merged := f.env[id.Name].or(values[i]); merged != f.env[id.Name] {
			f.env[id.Name] = merged
			grew = true
		}
	}
	return grew
}

func (f *funcFlow) resultsOf(exprs []ast.Expr) []rooting {
	if len(exprs) == 1 {
		return f.results(exprs[0])
	}
	out := make([]rooting, len(exprs))
	for i, e := range exprs {
		out[i] = f.of(e)
	}
	return out
}

func (f *funcFlow) results(expr ast.Expr) []rooting {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return []rooting{f.of(expr)}
	}
	if qualified(call) == "runtime.Caller" {
		return []rooting{{}, {tree: true}}
	}
	if results := f.localResults(call); results != nil {
		return results
	}
	return []rooting{f.ofCall(call)}
}

func (f *funcFlow) localResults(call *ast.CallExpr) []rooting {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}
	fn, local := f.scan.funcs[id.Name]
	if !local {
		return nil
	}
	out := make([]rooting, len(f.scan.returns[id.Name]))
	for i, r := range f.scan.returns[id.Name] {
		out[i] = f.substitute(fn, call, r)
	}
	return out
}

func (f *funcFlow) substitute(fn *ast.FuncDecl, call *ast.CallExpr, r rooting) rooting {
	out := rooting{tree: r.tree}
	for j := range 64 {
		if r.params&(1<<j) != 0 {
			out = out.or(f.argument(fn, call, j))
		}
	}
	return out
}

func (f *funcFlow) argument(fn *ast.FuncDecl, call *ast.CallExpr, j int) rooting {
	last := len(paramNames(fn.Type)) - 1
	var r rooting
	for k, arg := range call.Args {
		if k == j || (j == last && k > last && isVariadic(fn.Type)) {
			r = r.or(f.of(arg))
		}
	}
	return r
}

func qualified(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if pkg, ok := sel.X.(*ast.Ident); ok {
		return pkg.Name + "." + sel.Sel.Name
	}
	return ""
}

func (f *funcFlow) of(expr ast.Expr) rooting {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if f.shadowed[e] {
			return f.env[e.Name]
		}
		return f.env[e.Name].or(f.scan.globals[e.Name])
	case *ast.BinaryExpr:
		return f.of(e.X).or(f.of(e.Y))
	case *ast.IndexExpr:
		return f.of(e.X)
	case *ast.CompositeLit:
		return f.union(e.Elts)
	case *ast.CallExpr:
		return f.ofCall(e)
	}
	return rooting{}
}

func (f *funcFlow) ofCall(call *ast.CallExpr) rooting {
	if steps, _ := climb(call); steps > 0 {
		return rooting{tree: true}
	}
	if asksGitForTheTopLevel(call) {
		return rooting{tree: allLiterals(call.Args)}.or(f.union(call.Args))
	}
	switch q := qualified(call); {
	case q == "os.Getwd":
		return f.workingDirectoryAt(call)
	case q == "filepath.Join" || q == "path.Join" || q == "fmt.Sprintf":
		return f.union(call.Args)
	case slices.Contains(passPathThrough, q) && len(call.Args) > 0:
		return f.of(call.Args[0])
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Output" || sel.Sel.Name == "CombinedOutput") {
		return f.of(sel.X)
	}
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "string" && len(call.Args) == 1 {
		return f.of(call.Args[0])
	}
	if results := f.localResults(call); len(results) > 0 {
		return results[0]
	}
	return rooting{}
}

func asksGitForTheTopLevel(call *ast.CallExpr) bool {
	return slices.ContainsFunc(call.Args, func(arg ast.Expr) bool {
		text := stringLiteral(arg)
		return text != nil && *text == "--show-toplevel"
	})
}

func allLiterals(args []ast.Expr) bool {
	return !slices.ContainsFunc(args, func(arg ast.Expr) bool { return stringLiteral(arg) == nil })
}

func (f *funcFlow) union(args []ast.Expr) rooting {
	var r rooting
	for _, a := range args {
		r = r.or(f.of(a))
	}
	return r
}

func (f *funcFlow) sinks(body ast.Node, report func(*ast.CallExpr)) (writes uint64) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, target := range f.writeTargets(call) {
			writes |= target.params
			if target.tree && report != nil {
				report(call)
			}
		}
		return true
	})
	return writes
}

func (f *funcFlow) writeTargets(call *ast.CallExpr) []rooting {
	var targets []rooting
	pkg, name, _ := strings.Cut(qualified(call), ".")
	if pkg == "os" || pkg == "ioutil" {
		for _, i := range osPathWrites[name] {
			if i < len(call.Args) {
				targets = append(targets, f.of(call.Args[i]))
			}
		}
		if name == "OpenFile" && len(call.Args) > 1 && opensForWriting(call.Args[1]) {
			targets = append(targets, f.of(call.Args[0]))
		}
	}
	if id, ok := call.Fun.(*ast.Ident); ok {
		if fn, local := f.scan.funcs[id.Name]; local {
			targets = append(targets, f.substitute(fn, call, rooting{params: f.scan.writes[id.Name]}))
		}
	}
	return targets
}

func opensForWriting(flags ast.Expr) bool {
	found := false
	ast.Inspect(flags, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && slices.Contains(openWriteFlags, sel.Sel.Name) {
			found = true
		}
		return !found
	})
	return found
}
