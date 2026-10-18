package gotypes

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

const initCaller = "<init>"

type siteKey struct {
	callee int
	value  string
	site   token.Position
}

type indexer struct {
	fset     *token.FileSet
	root     string
	data     indexData
	fileIDs  map[string]int
	funcIDs  map[token.Position]int
	initIDs  map[string]int
	funcObjs []*types.Func
	modules  map[string]bool
	seen     map[siteKey]bool
}

func newIndexer(fset *token.FileSet, root string, modules map[string]bool) *indexer {
	return &indexer{
		fset:    fset,
		root:    root,
		data:    indexData{Version: indexVersion, Decls: map[int][]declRecord{}},
		fileIDs: map[string]int{},
		funcIDs: map[token.Position]int{},
		initIDs: map[string]int{},
		modules: modules,
		seen:    map[siteKey]bool{},
	}
}

func (x *indexer) addPackage(pkg *types.Package, info *types.Info, files []*ast.File) {
	for _, f := range files {
		x.defineFile(info, f)
	}
	for _, f := range files {
		x.callsOfFile(pkg, info, f)
	}
}

func (x *indexer) finish(warnings []string) indexData {
	x.linkImplements()
	slices.SortStableFunc(x.data.Calls, func(a, b callRecord) int {
		return cmp.Or(strings.Compare(x.data.Files[a.File], x.data.Files[b.File]), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
	x.data.Warnings = warnings
	return x.data
}

func (x *indexer) fileID(filename string) int {
	if id, ok := x.fileIDs[filename]; ok {
		return id
	}
	rel, ok := strings.CutPrefix(filename, x.root+"/")
	if !ok {
		rel = filename
	}
	id := len(x.data.Files)
	x.fileIDs[filename] = id
	x.data.Files = append(x.data.Files, rel)
	return id
}

func (x *indexer) funcID(fn *types.Func) int {
	pos := x.fset.Position(fn.Pos())
	if id, ok := x.funcIDs[pos]; ok {
		return id
	}
	rec := funcRecord{Package: packageName(fn), Name: fn.Name(), File: x.fileID(pos.Filename), Line: pos.Line, Module: x.modules[packagePath(fn)]}
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		rec.Recv = typeName(recv.Type())
		_, rec.Interface = recv.Type().Underlying().(*types.Interface)
	}
	id := len(x.data.Funcs)
	x.funcIDs[pos] = id
	x.data.Funcs = append(x.data.Funcs, rec)
	x.funcObjs = append(x.funcObjs, fn)
	return id
}

func packageName(fn *types.Func) string {
	if fn.Pkg() == nil {
		return "builtin"
	}
	return fn.Pkg().Name()
}

func packagePath(fn *types.Func) string {
	if fn.Pkg() == nil {
		return ""
	}
	return fn.Pkg().Path()
}

func typeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Obj().Name()
	}
	return "interface"
}

func (x *indexer) defineFile(info *types.Info, f *ast.File) {
	file := x.fileID(x.fset.Position(f.Pos()).Filename)
	_, declared := x.data.Decls[file]
	var decls []declRecord
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			x.funcID(info.Defs[d.Name].(*types.Func))
			decls = append(decls, x.funcDecl(d))
		case *ast.GenDecl:
			decls = append(decls, x.genDecl(info, d)...)
		}
	}
	if !declared {
		x.data.Decls[file] = decls
	}
}

func (x *indexer) funcDecl(d *ast.FuncDecl) declRecord {
	line := x.fset.Position(d.Pos()).Line
	if d.Recv == nil {
		return declRecord{Name: d.Name.Name, Kind: "func", Line: line}
	}
	return declRecord{Name: recvName(d.Recv.List[0].Type) + "." + d.Name.Name, Kind: "method", Line: line}
}

func recvName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return recvName(e.X)
	case *ast.IndexExpr:
		return recvName(e.X)
	case *ast.IndexListExpr:
		return recvName(e.X)
	}
	return e.(*ast.Ident).Name
}

func (x *indexer) genDecl(info *types.Info, d *ast.GenDecl) []declRecord {
	var decls []declRecord
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			decls = append(decls, declRecord{Name: s.Name.Name, Kind: "type", Line: x.fset.Position(s.Pos()).Line})
			x.defineInterface(info, s)
		case *ast.ValueSpec:
			for _, n := range s.Names {
				decls = append(decls, declRecord{Name: n.Name, Kind: d.Tok.String(), Line: x.fset.Position(n.Pos()).Line})
			}
		}
	}
	return decls
}

func (x *indexer) defineInterface(info *types.Info, s *ast.TypeSpec) {
	it, ok := s.Type.(*ast.InterfaceType)
	if !ok {
		return
	}
	for _, m := range it.Methods.List {
		for _, n := range m.Names {
			x.funcID(info.Defs[n].(*types.Func))
		}
	}
}

func (x *indexer) callsOfFile(pkg *types.Package, info *types.Info, f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				x.callsIn(info, d.Body, x.funcID(info.Defs[d.Name].(*types.Func)))
			}
		case *ast.GenDecl:
			x.callsOfValues(pkg, info, d)
		}
	}
}

func (x *indexer) callsOfValues(pkg *types.Package, info *types.Info, d *ast.GenDecl) {
	for _, spec := range d.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok {
			for _, v := range vs.Values {
				x.callsIn(info, v, x.initID(pkg, vs))
			}
		}
	}
}

func (x *indexer) initID(pkg *types.Package, at ast.Node) int {
	if id, ok := x.initIDs[pkg.Path()]; ok {
		return id
	}
	pos := x.fset.Position(at.Pos())
	id := len(x.data.Funcs)
	x.initIDs[pkg.Path()] = id
	x.data.Funcs = append(x.data.Funcs, funcRecord{Package: pkg.Name(), Name: initCaller, File: x.fileID(pos.Filename), Line: pos.Line, Module: true})
	x.funcObjs = append(x.funcObjs, nil)
	return id
}

func (x *indexer) callsIn(info *types.Info, root ast.Node, caller int) {
	ast.Inspect(root, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			x.recordCall(info, call, caller)
		}
		return true
	})
}

func (x *indexer) recordCall(info *types.Info, call *ast.CallExpr, caller int) {
	id := calleeIdent(call.Fun)
	if id == nil {
		return
	}
	rec := callRecord{Callee: -1, Caller: caller}
	switch obj := info.Uses[id].(type) {
	case *types.Func:
		fn := obj.Origin()
		if !x.modules[packagePath(fn)] && !isInterfaceMethod(fn) {
			return
		}
		rec.Callee = x.funcID(fn)
		if x.data.Funcs[rec.Callee].Interface {
			rec.Kind = kindInterface
		}
	case *types.Var:
		rec.Value, rec.Kind = obj.Name(), kindFuncValue
	default:
		return
	}
	site := x.fset.Position(call.Lparen)
	key := siteKey{callee: rec.Callee, value: rec.Value, site: site}
	if x.seen[key] {
		return
	}
	x.seen[key] = true
	rec.File, rec.Line, rec.Column = x.fileID(site.Filename), site.Line, site.Column
	x.data.Calls = append(x.data.Calls, rec)
}

func isInterfaceMethod(fn *types.Func) bool {
	recv := fn.Type().(*types.Signature).Recv()
	if recv == nil {
		return false
	}
	_, ok := recv.Type().Underlying().(*types.Interface)
	return ok
}

func calleeIdent(e ast.Expr) *ast.Ident {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	case *ast.IndexExpr:
		return calleeIdent(e.X)
	case *ast.IndexListExpr:
		return calleeIdent(e.X)
	}
	return nil
}

func (x *indexer) linkImplements() {
	byName := map[string][]int{}
	for id, f := range x.data.Funcs {
		if f.Interface {
			byName[f.Name] = append(byName[f.Name], id)
		}
	}
	for id, f := range x.data.Funcs {
		if f.Recv == "" || f.Interface || !f.Module {
			continue
		}
		for _, iface := range byName[f.Name] {
			if implements(x.funcObjs[id], x.funcObjs[iface]) {
				x.data.Implements = append(x.data.Implements, implementsRecord{Concrete: id, Interface: iface})
			}
		}
	}
}

func implements(concrete, method *types.Func) bool {
	it := method.Type().(*types.Signature).Recv().Type().Underlying().(*types.Interface)
	recv := concrete.Type().(*types.Signature).Recv().Type()
	if p, ok := recv.(*types.Pointer); ok {
		recv = p.Elem()
	}
	return types.Implements(recv, it) || types.Implements(types.NewPointer(recv), it)
}
