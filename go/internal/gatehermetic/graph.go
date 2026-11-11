package gatehermetic

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
)

type Root struct {
	Package string
	Func    string
}

type Source struct {
	ModuleDir string
	Roots     []Root
}

type source struct {
	pkg       *goload.Package
	nodes     []ast.Node
	callsOnly bool
	sig       *types.Signature
}

type pkgInfo struct {
	pkg           *goload.Package
	inits         []string
	pkgInit       string
	bodyless      token.Pos
	exclusion     string
	exclusionDone bool
}

type Graph struct {
	program  goload.Program
	roots    []Root
	rootKeys []string
	pkgs     map[string]*pkgInfo
	sources  map[string]source
	assigned map[string]bool
	targets  map[ast.Node]types.Type
	execStd  map[string]bool
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

func RootOf(fn any) (Root, error) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || v.IsNil() {
		return Root{}, errors.New("gatehermetic: the root is not a function")
	}
	name := runtime.FuncForPC(v.Pointer()).Name()
	slash := strings.LastIndex(name, "/")
	pkg, fname, ok := strings.Cut(name[slash+1:], ".")
	if !ok || fname == "" || strings.ContainsAny(fname, ".()[]*") {
		return Root{}, fmt.Errorf("gatehermetic: %q is a closure or a method, not a named function", name)
	}
	return Root{Package: strings.ReplaceAll(name[:slash+1]+pkg, "%2e", "."), Func: fname}, nil
}

func Load(ctx context.Context, loader goload.Loader, src Source) (Graph, error) {
	if len(src.Roots) == 0 {
		return Graph{}, errors.New("gatehermetic: the source has no root")
	}
	prog, err := loader.Load(ctx, goload.Request{Dir: src.ModuleDir, Patterns: patternsOf(src.Roots), Env: os.Environ()})
	if err != nil {
		return Graph{}, fmt.Errorf("gatehermetic: %w", err)
	}
	g := Graph{
		program:  prog,
		roots:    slices.Clone(src.Roots),
		pkgs:     map[string]*pkgInfo{},
		sources:  map[string]source{},
		assigned: map[string]bool{},
		targets:  map[ast.Node]types.Type{},
	}
	if err := g.index(); err != nil {
		return Graph{}, err
	}
	if err := g.bindRoots(); err != nil {
		return Graph{}, err
	}
	return g, nil
}

func patternsOf(roots []Root) []string {
	var out []string
	for _, r := range roots {
		out = append(out, r.Package)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (g *Graph) index() error {
	checked := map[string]*types.Package{}
	for _, path := range g.program.Order {
		p := g.program.Packages[path]
		if p.Standard {
			continue
		}
		if p.Module && len(p.Errors) > 0 {
			return fmt.Errorf("gatehermetic: %s does not type-check: %w", path, p.Errors[0])
		}
		var err error
		if p, err = recheck(g.program, p, checked); err != nil {
			return err
		}
		checked[path] = p.Types
		g.addPackage(p)
	}
	g.collectAssigned()
	g.execStd = execClosure(g.program)
	return nil
}

func execClosure(prog goload.Program) map[string]bool {
	memo := map[string]bool{}
	var holds func(path string) bool
	holds = func(path string) bool {
		if v, ok := memo[path]; ok {
			return v
		}
		memo[path] = false
		if p := prog.Packages[path]; p != nil {
			memo[path] = slices.ContainsFunc(p.Types.Imports(), func(imp *types.Package) bool { return imp.Path() == "os/exec" || holds(imp.Path()) })
		}
		return memo[path]
	}
	out := map[string]bool{}
	for path, p := range prog.Packages {
		if p.Standard && holds(path) {
			out[path] = true
		}
	}
	return out
}

func recheck(prog goload.Program, p *goload.Package, done map[string]*types.Package) (*goload.Package, error) {
	files, errs, err := syntaxOf(prog, p)
	if err != nil {
		return nil, err
	}
	info := &types.Info{
		Defs:      map[*ast.Ident]types.Object{},
		Uses:      map[*ast.Ident]types.Object{},
		Types:     map[ast.Expr]types.TypeAndValue{},
		Instances: map[*ast.Ident]types.Instance{},
	}
	conf := types.Config{
		Importer: importerFunc(func(path string) (*types.Package, error) {
			if path == "unsafe" {
				return types.Unsafe, nil
			}
			if t, ok := done[path]; ok {
				return t, nil
			}
			if q, ok := prog.Packages[path]; ok {
				return q.Types, nil
			}
			return nil, fmt.Errorf("gatehermetic: go list did not list the import %s", path)
		}),
		FakeImportC: true,
		Error:       func(e error) { errs = append(errs, e) },
	}
	pkg, _ := conf.Check(p.Path, prog.Fset, files, info)
	if len(errs) > 0 {
		return nil, fmt.Errorf("gatehermetic: %s does not type-check: %w", p.Path, errs[0])
	}
	return &goload.Package{Path: p.Path, Dir: p.Dir, Types: pkg, Info: info, Files: files}, nil
}

func syntaxOf(prog goload.Program, p *goload.Package) ([]*ast.File, []error, error) {
	if p.Module {
		return p.Files, nil, nil
	}
	bp, err := importDir(p.Dir)
	if err != nil {
		return nil, nil, fmt.Errorf("gatehermetic: read the files of %s: %w", p.Path, err)
	}
	var files []*ast.File
	var errs []error
	for _, name := range bp.GoFiles {
		f, perr := parser.ParseFile(prog.Fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			errs = append(errs, perr)
			continue
		}
		files = append(files, f)
	}
	return files, errs, nil
}

func importDir(dir string) (*build.Package, error) {
	ctxt := build.Default
	ctxt.CgoEnabled = false
	return ctxt.ImportDir(dir, 0)
}

func excludedFile(dir string) string {
	bp, _ := importDir(dir)
	names := slices.DeleteFunc(slices.Concat(bp.IgnoredGoFiles, bp.InvalidGoFiles, bp.CgoFiles), func(n string) bool {
		return strings.HasSuffix(n, "_test.go")
	})
	if len(names) == 0 {
		return ""
	}
	slices.Sort(names)
	return filepath.Join(dir, names[0])
}

func (pi *pkgInfo) excluded() string {
	if !pi.exclusionDone {
		pi.exclusion, pi.exclusionDone = excludedFile(pi.pkg.Dir), true
	}
	return pi.exclusion
}

func (g *Graph) addPackage(p *goload.Package) {
	pi := &pkgInfo{pkg: p}
	g.pkgs[p.Path] = pi
	var initNodes []ast.Node
	for _, f := range p.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				g.addFunc(pi, x)
			case *ast.GenDecl:
				initNodes = append(initNodes, g.addVars(p, x)...)
			}
		}
	}
	if len(initNodes) > 0 {
		pi.pkgInit = p.Path + ".<initializers>"
		g.sources[pi.pkgInit] = source{pkg: p, nodes: initNodes, callsOnly: true}
	}
}

func (g *Graph) addFunc(pi *pkgInfo, fd *ast.FuncDecl) {
	if fd.Body == nil {
		if !pi.bodyless.IsValid() {
			pi.bodyless = fd.Pos()
		}
		return
	}
	src := source{pkg: pi.pkg, nodes: []ast.Node{fd.Body}}
	if fn, ok := pi.pkg.Info.Defs[fd.Name].(*types.Func); ok {
		src.sig, _ = fn.Type().(*types.Signature)
	}
	if fd.Recv == nil && fd.Name.Name == "init" {
		key := fmt.Sprintf("%s.init#%d", pi.pkg.Path, len(pi.inits))
		pi.inits = append(pi.inits, key)
		g.sources[key] = src
		return
	}
	if fn, ok := pi.pkg.Info.Defs[fd.Name].(*types.Func); ok {
		g.sources[fn.FullName()] = src
	}
}

func (g *Graph) addVars(p *goload.Package, d *ast.GenDecl) []ast.Node {
	var all []ast.Node
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || d.Tok != token.VAR {
			continue
		}
		for i, name := range vs.Names {
			exprs := vs.Values
			if len(vs.Values) == len(vs.Names) {
				exprs = vs.Values[i : i+1]
			}
			if len(exprs) == 0 {
				continue
			}
			nodes := make([]ast.Node, len(exprs))
			for j, e := range exprs {
				nodes[j] = e
			}
			if vs.Type != nil && len(exprs) == 1 {
				g.targets[exprs[0]] = p.Info.TypeOf(vs.Type)
			}
			g.sources[p.Path+".var "+name.Name] = source{pkg: p, nodes: nodes}
			all = append(all, nodes...)
		}
	}
	return all
}

func isPkgVar(o types.Object) (*types.Var, bool) {
	v, ok := o.(*types.Var)
	return v, ok && !v.IsField() && v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

func varKey(v *types.Var) string { return v.Pkg().Path() + ".var " + v.Name() }

func (g *Graph) collectAssigned() {
	for _, pi := range g.pkgs {
		for _, f := range pi.pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					for _, l := range x.Lhs {
						g.markAssigned(pi.pkg.Info, l)
					}
				case *ast.IncDecStmt:
					g.markAssigned(pi.pkg.Info, x.X)
				case *ast.RangeStmt:
					g.markAssigned(pi.pkg.Info, x.Key)
					g.markAssigned(pi.pkg.Info, x.Value)
				case *ast.UnaryExpr:
					if x.Op == token.AND {
						g.markAssigned(pi.pkg.Info, x.X)
					}
				}
				return true
			})
		}
	}
}

func (g *Graph) markAssigned(info *types.Info, e ast.Expr) {
	for e != nil {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.SelectorExpr:
			if v, ok := isPkgVar(info.Uses[x.Sel]); ok {
				g.assigned[varKey(v)] = true
				return
			}
			e = x.X
		case *ast.Ident:
			if v, ok := isPkgVar(info.Uses[x]); ok {
				g.assigned[varKey(v)] = true
			}
			return
		default:
			return
		}
	}
}

func (g *Graph) bindRoots() error {
	for _, r := range g.roots {
		pi := g.pkgs[r.Package]
		if pi == nil {
			return fmt.Errorf("gatehermetic: the loaded program has no package %s for the root %s", r.Package, r.Func)
		}
		fn, _ := pi.pkg.Types.Scope().Lookup(r.Func).(*types.Func)
		if fn == nil {
			return fmt.Errorf("gatehermetic: the package %s does not declare the root %s", r.Package, r.Func)
		}
		if _, ok := g.sources[fn.FullName()]; !ok {
			return fmt.Errorf("gatehermetic: the root %s.%s has no body", r.Package, r.Func)
		}
		g.rootKeys = append(g.rootKeys, fn.FullName())
	}
	return nil
}
