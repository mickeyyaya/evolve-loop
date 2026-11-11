package gatehermetic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

type finding struct {
	class string
	pos   token.Pos
}

type edge struct {
	key  string
	kind string
}

type varRead struct {
	key string
	pkg string
}

type execCall struct {
	pos   token.Pos
	fn    string
	prog  string
	known bool
}

type fieldUse struct {
	pos token.Pos
	fn  string
}

type watchUse struct {
	pos  token.Pos
	call *ast.CallExpr
}

type facts struct {
	findings []finding
	edges    []edge
	vars     []varRead
	types    []types.Type
	pkgs     []string
	execs    []execCall
	fields   []fieldUse
	watched  []watchUse
}

type scanner struct {
	g         *Graph
	rules     *Rules
	info      *types.Info
	out       *facts
	fn        string
	callsOnly bool
	words     bool
	watch     *types.Func
	sig       *types.Signature
	stack     []ast.Node
	skip      int
	flows     map[*ast.Ident]bool
	invoked   map[*ast.FuncLit]bool
	runner    *runnerType
}

type runnerType struct {
	entry ExecEntry
	sig   *types.Signature
}

func (s *scanner) add(class string, pos token.Pos) {
	s.out.findings = append(s.out.findings, finding{class: class, pos: pos})
}

func (s *scanner) scan(root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			if s.skip == len(s.stack) {
				s.skip = 0
			}
			s.stack = s.stack[:len(s.stack)-1]
			return true
		}
		if lit, ok := n.(*ast.FuncLit); ok && s.initOnly() && !s.invoked[lit] {
			return false
		}
		s.stack = append(s.stack, n)
		s.constCheck(n)
		s.conversions(n)
		switch x := n.(type) {
		case *ast.Ident:
			s.ident(x)
		case *ast.CallExpr:
			s.markInvoked(x)
			s.call(x)
		case *ast.CompositeLit:
			if s.isCmdType(x.Type) {
				s.cmdSite(x.Pos())
			}
		}
		return true
	})
}

func (s *scanner) markInvoked(c *ast.CallExpr) {
	if lit, ok := ast.Unparen(c.Fun).(*ast.FuncLit); ok {
		if s.invoked == nil {
			s.invoked = map[*ast.FuncLit]bool{}
		}
		s.invoked[lit] = true
	}
}

func (s *scanner) inInvoked() bool {
	return slices.ContainsFunc(s.stack, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		return ok && s.invoked[lit]
	})
}

func (s *scanner) initOnly() bool { return s.callsOnly && !s.inInvoked() }

func (s *scanner) cmdSite(pos token.Pos) {
	s.add(classExec, pos)
	s.out.execs = append(s.out.execs, execCall{pos: pos, fn: s.fn})
}

func (s *scanner) constCheck(n ast.Node) {
	e, ok := n.(ast.Expr)
	if !ok || s.skip != 0 {
		return
	}
	str, ok := constString(s.info, e)
	if !ok {
		return
	}
	s.skip = len(s.stack)
	if slices.Contains(strings.Split(str, "/"), ".git") {
		s.add(classGitDir, n.Pos())
	}
	if s.words && slices.Contains(s.rules.GitWords, str) {
		s.add(classGitWord, n.Pos())
	}
}

func constString(info *types.Info, e ast.Expr) (string, bool) {
	v := constValue(info, e)
	if v == nil || v.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(v), true
}

func constValue(info *types.Info, e ast.Expr) constant.Value {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return constant.MakeFromLiteral(x.Value, x.Kind, 0)
		}
	case *ast.ParenExpr:
		return constValue(info, x.X)
	case *ast.Ident:
		return constOf(info.Uses[x])
	case *ast.SelectorExpr:
		return constOf(info.Uses[x.Sel])
	case *ast.BinaryExpr:
		l, r := constValue(info, x.X), constValue(info, x.Y)
		if x.Op == token.ADD && l != nil && r != nil && l.Kind() == constant.String && r.Kind() == constant.String {
			return constant.BinaryOp(l, token.ADD, r)
		}
	}
	return nil
}

func constOf(o types.Object) constant.Value {
	if c, ok := o.(*types.Const); ok {
		return c.Val()
	}
	return nil
}

func (s *scanner) ident(id *ast.Ident) {
	if inst, ok := s.info.Instances[id]; ok {
		for i := range inst.TypeArgs.Len() {
			s.out.types = append(s.out.types, inst.TypeArgs.At(i))
		}
	}
	switch o := s.info.Uses[id].(type) {
	case *types.Func:
		s.funcUse(id, o.Origin())
	case *types.Var:
		s.varUse(id, o)
	case *types.Const:
		if o.Pkg() != nil {
			s.out.pkgs = append(s.out.pkgs, o.Pkg().Path())
		}
	}
}

func (s *scanner) runnerOf() *runnerType {
	if s.runner != nil {
		return s.runner
	}
	s.runner = &runnerType{}
	for _, e := range s.rules.Exec {
		pi := s.g.pkgs[e.Package]
		if pi == nil {
			continue
		}
		if tn, ok := pi.pkg.Types.Scope().Lookup(e.Name).(*types.TypeName); ok {
			if sig, ok := tn.Type().Underlying().(*types.Signature); ok {
				s.runner = &runnerType{entry: e, sig: sig}
				break
			}
		}
	}
	return s.runner
}

func headOf(parent, cur ast.Node) bool {
	switch p := parent.(type) {
	case *ast.SelectorExpr:
		return p.Sel == cur
	case *ast.ParenExpr:
		return p.X == cur
	case *ast.IndexExpr:
		return p.X == cur
	case *ast.IndexListExpr:
		return p.X == cur
	}
	return false
}

func enclosingCall(stack []ast.Node) (*ast.CallExpr, bool) {
	cur := stack[len(stack)-1]
	for i := len(stack) - 2; i >= 0; i-- {
		if c, ok := stack[i].(*ast.CallExpr); ok {
			return c, c.Fun == cur
		}
		if !headOf(stack[i], cur) {
			return nil, false
		}
		cur = stack[i]
	}
	return nil, false
}

func (s *scanner) funcUse(id *ast.Ident, fn *types.Func) {
	pkg := fn.Pkg()
	if pkg == nil {
		return
	}
	sig := fn.Type().(*types.Signature)
	enclosing, isCallee := enclosingCall(s.stack)
	var call *ast.CallExpr
	if isCallee {
		call = enclosing
	}
	if s.watch != nil && fn == s.watch {
		s.out.watched = append(s.out.watched, watchUse{pos: id.Pos(), call: call})
	}
	if sig.Recv() == nil {
		s.listed(id, pkg.Path()+"."+fn.Name())
	} else if isStatMethod(fn, sig) {
		s.add(classMtime, id.Pos())
	}
	if call == nil && s.execLike(fn) && !s.flows[id] {
		s.add(classExec, id.Pos())
	}
	if (sig.Recv() != nil && types.IsInterface(sig.Recv().Type())) || (s.initOnly() && enclosing == nil) {
		return
	}
	kind := "value"
	if call != nil {
		kind = "call"
	}
	s.out.edges = append(s.out.edges, edge{key: fn.FullName(), kind: kind})
}

func (s *scanner) listed(id *ast.Ident, name string) {
	if slices.Contains(s.rules.Mtime, name) {
		s.add(classMtime, id.Pos())
	}
	if slices.Contains(s.rules.History, name) {
		s.add(classHistory, id.Pos())
	}
}

func isStatMethod(fn *types.Func, sig *types.Signature) bool {
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	res := sig.Results().At(0).Type()
	switch fn.Name() {
	case "ModTime":
		n, ok := types.Unalias(res).(*types.Named)
		return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "time" && n.Obj().Name() == "Time"
	case "Sys":
		i, ok := res.Underlying().(*types.Interface)
		return ok && i.Empty()
	}
	return false
}

func (s *scanner) varUse(id *ast.Ident, v *types.Var) {
	if v.IsField() {
		if p := v.Pkg(); p != nil && p.Path() == "os/exec" && (v.Name() == "Path" || v.Name() == "Args") {
			s.add(classExec, id.Pos())
			s.out.fields = append(s.out.fields, fieldUse{pos: id.Pos(), fn: s.fn})
		}
		return
	}
	if _, ok := isPkgVar(v); !ok || s.isPlainWrite(id) {
		return
	}
	key := varKey(v)
	s.out.vars = append(s.out.vars, varRead{key: key, pkg: v.Pkg().Path()})
	if s.g.assigned[key] && holdsCode(v.Type(), map[types.Type]bool{}) {
		s.add(classInject, id.Pos())
	}
}

func (s *scanner) isPlainWrite(id *ast.Ident) bool {
	if len(s.stack) < 2 {
		return false
	}
	a, ok := s.stack[len(s.stack)-2].(*ast.AssignStmt)
	return ok && a.Tok == token.ASSIGN && slices.ContainsFunc(a.Lhs, func(e ast.Expr) bool { return e == ast.Expr(id) })
}

func holdsCode(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch x := t.(type) {
	case *types.Named:
		return holdsCode(x.Underlying(), seen)
	case *types.Alias:
		return holdsCode(types.Unalias(x), seen)
	case *types.Signature, *types.Interface, *types.Chan:
		return true
	case *types.Pointer:
		return holdsCode(x.Elem(), seen)
	case *types.Slice:
		return holdsCode(x.Elem(), seen)
	case *types.Array:
		return holdsCode(x.Elem(), seen)
	case *types.Map:
		return holdsCode(x.Key(), seen) || holdsCode(x.Elem(), seen)
	case *types.Struct:
		for i := range x.NumFields() {
			if holdsCode(x.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

func calleeIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.SelectorExpr:
			return x.Sel
		case *ast.Ident:
			return x
		default:
			return nil
		}
	}
}

func (s *scanner) isCmdType(e ast.Expr) bool {
	if e == nil {
		return false
	}
	id := calleeIdent(e)
	if id == nil {
		return false
	}
	tn, ok := s.info.Uses[id].(*types.TypeName)
	return ok && tn.Pkg() != nil && tn.Pkg().Path() == "os/exec" && tn.Name() == "Cmd"
}

func (s *scanner) call(c *ast.CallExpr) {
	if s.info.Types[c.Fun].IsType() {
		return
	}
	if id := calleeIdent(c.Fun); id != nil {
		switch o := s.info.Uses[id].(type) {
		case *types.Builtin:
			if o.Name() == "new" && len(c.Args) == 1 && s.isCmdType(c.Args[0]) {
				s.cmdSite(c.Pos())
			}
			return
		case *types.Func:
			s.funcCall(c, o.Origin())
			return
		}
	}
	s.valueCall(c)
}

func (s *scanner) entry(pkg, name string) (ExecEntry, bool) {
	i := slices.IndexFunc(s.rules.Exec, func(e ExecEntry) bool { return e.Package == pkg && e.Name == name })
	if i < 0 {
		return ExecEntry{}, false
	}
	return s.rules.Exec[i], true
}

func namedOf(t types.Type) *types.Named {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, _ := t.(*types.Named)
	return n
}

func methodName(fn *types.Func) string {
	name := fn.Name()
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		if n := namedOf(recv.Type()); n != nil {
			name = n.Obj().Name() + "." + name
		}
	}
	return name
}

func (s *scanner) execLike(fn *types.Func) bool {
	path := fn.Pkg().Path()
	if _, ok := s.entry(path, methodName(fn)); ok {
		return true
	}
	if slices.Contains(s.rules.Inert, path+"."+fn.Name()) {
		return false
	}
	return slices.Contains(s.rules.Exempt, path) || s.g.execStd[path]
}

func (s *scanner) funcCall(c *ast.CallExpr, fn *types.Func) {
	if fn.Pkg() == nil {
		return
	}
	if e, ok := s.entry(fn.Pkg().Path(), methodName(fn)); ok {
		s.execCall(c, e, fn.Type().(*types.Signature))
		return
	}
	if s.execLike(fn) {
		s.add(classExec, c.Pos())
	}
}

func (s *scanner) valueCall(c *ast.CallExpr) {
	if _, lit := ast.Unparen(c.Fun).(*ast.FuncLit); lit {
		return
	}
	r := s.runnerOf()
	if r.sig == nil {
		return
	}
	if s.isRunnerValue(s.info.TypeOf(c.Fun)) {
		s.execCall(c, r.entry, r.sig)
	}
}

func (s *scanner) isRunnerValue(t types.Type) bool {
	r := s.runnerOf()
	return r.sig != nil && slices.ContainsFunc(signaturesOf(t), func(sig *types.Signature) bool { return types.Identical(sig, r.sig) })
}

func signaturesOf(t types.Type) []*types.Signature {
	switch u := t.Underlying().(type) {
	case *types.Signature:
		return []*types.Signature{u}
	case *types.Interface:
		var out []*types.Signature
		for i := range u.NumEmbeddeds() {
			out = append(out, embeddedSignatures(u.EmbeddedType(i))...)
		}
		return out
	}
	return nil
}

func embeddedSignatures(t types.Type) []*types.Signature {
	union, ok := t.(*types.Union)
	if !ok {
		return signaturesOf(t)
	}
	var out []*types.Signature
	for i := range union.Len() {
		out = append(out, signaturesOf(union.Term(i).Type())...)
	}
	return out
}

func paramIndex(sig *types.Signature, name string) int {
	for i := range sig.Params().Len() {
		if sig.Params().At(i).Name() == name {
			return i
		}
	}
	return -1
}

func (s *scanner) execCall(c *ast.CallExpr, e ExecEntry, sig *types.Signature) {
	var prog string
	var known bool
	if i := paramIndex(sig, e.Param); e.Param != "" && i >= 0 && i < len(c.Args) {
		prog, known = constString(s.info, c.Args[i])
	}
	class := classExec
	if known && prog == "git" {
		class = classGit
	}
	s.add(class, c.Pos())
	s.out.execs = append(s.out.execs, execCall{pos: c.Pos(), fn: s.fn, prog: prog, known: known})
}

func scanPackage(g *Graph, pi *pkgInfo, rules *Rules, watch *types.Func) *facts {
	out := &facts{}
	for _, f := range pi.pkg.Files {
		for _, d := range f.Decls {
			s := &scanner{g: g, rules: rules, info: pi.pkg.Info, out: out, words: true, watch: watch}
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := pi.pkg.Info.Defs[fd.Name].(*types.Func); ok {
					s.fn = fn.FullName()
				}
			}
			s.scan(d)
		}
	}
	return out
}

func Audit(g Graph, pkg string, rules Rules) []Refusal {
	pi := g.pkgs[pkg]
	if pi == nil {
		return []Refusal{{Class: "absent", Site: pkg}}
	}
	f := scanPackage(&g, pi, &rules, nil)
	return auditRefusals(g, pkg, f, classMtime, classGitDir, classHistory, classGitWord)
}

func auditRefusals(g Graph, pkg string, f *facts, classes ...string) []Refusal {
	var out []Refusal
	for _, fd := range f.findings {
		if slices.Contains(classes, fd.class) {
			out = append(out, Refusal{Root: Root{Package: pkg}, Class: fd.class, Site: g.program.Fset.Position(fd.pos).String()})
		}
	}
	sortRefusals(out)
	return out
}

func sortRefusals(r []Refusal) {
	slices.SortStableFunc(r, func(a, b Refusal) int { return strings.Compare(a.Class+a.Site, b.Class+b.Site) })
}
