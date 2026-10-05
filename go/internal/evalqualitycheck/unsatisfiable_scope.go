package evalqualitycheck

import (
	"go/ast"
	"go/token"
)

type nameKind int

const (
	kindOther nameKind = iota
	kindConcreteTB
	kindInterfaceTB
	kindGoRunCommand
	kindGoRunError
	kindGoRunExitError
)

type binding struct {
	kind         nameKind
	goFlagsGiven bool
}

type scope struct {
	names   map[string]binding
	closure bool
}

type scopeStack []scope

func (s *scopeStack) open(n ast.Node) {
	_, closure := n.(*ast.FuncLit)
	*s = append(*s, scope{names: map[string]binding{}, closure: closure})
}

func (s *scopeStack) pop() { *s = (*s)[:len(*s)-1] }

func (s scopeStack) declare(id *ast.Ident, b binding) {
	s[len(s)-1].names[id.Name] = b
}

func (s scopeStack) assign(id *ast.Ident, b binding) {
	frame := max(s.declaringFrame(id.Name), 0)
	if frame < s.innermostClosure() {
		s[frame].names[id.Name] = binding{}
		return
	}
	s[frame].names[id.Name] = b
}

func (s scopeStack) testingTB(expr ast.Expr) bool {
	b, _, found := s.find(expr)
	return (found && b.kind == kindConcreteTB) || s.valueOf(expr).kind == kindInterfaceTB
}

func (s scopeStack) valueOf(expr ast.Expr) binding {
	b, frame, found := s.find(expr)
	if !found || frame < s.innermostClosure() {
		return binding{}
	}
	return b
}

func (s scopeStack) find(expr ast.Expr) (binding, int, bool) {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return binding{}, -1, false
	}
	frame := s.declaringFrame(id.Name)
	if frame < 0 {
		return binding{}, -1, false
	}
	return s[frame].names[id.Name], frame, true
}

func (s scopeStack) declaringFrame(name string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if _, declared := s[i].names[name]; declared {
			return i
		}
	}
	return -1
}

func (s scopeStack) innermostClosure() int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i].closure {
			return i
		}
	}
	return -1
}

type scopedWalk struct {
	scopes      scopeStack
	opened      []bool
	predeclared map[ast.Node]bool
	visit       func(ast.Node, scopeStack)
}

func walkScoped(fn *ast.FuncDecl, visit func(ast.Node, scopeStack)) {
	w := &scopedWalk{predeclared: map[ast.Node]bool{}, visit: visit}
	w.scopes.open(fn)
	declareSignature(w.scopes, fn.Type)
	ast.Inspect(fn.Body, w.step)
}

func (w *scopedWalk) step(n ast.Node) bool {
	if n == nil {
		w.leave()
		return true
	}
	opens := opensScope(n)
	w.opened = append(w.opened, opens)
	if opens {
		w.scopes.open(n)
	}
	if !w.predeclared[n] {
		declareAt(n, w.scopes)
	}
	if ifs, ok := n.(*ast.IfStmt); ok && ifs.Init != nil {
		declareAt(ifs.Init, w.scopes)
		w.predeclared[ifs.Init] = true
	}
	w.visit(n, w.scopes)
	return true
}

func (w *scopedWalk) leave() {
	if w.opened[len(w.opened)-1] {
		w.scopes.pop()
	}
	w.opened = w.opened[:len(w.opened)-1]
}

func opensScope(n ast.Node) bool {
	switch n.(type) {
	case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
		*ast.TypeSwitchStmt, *ast.CaseClause, *ast.CommClause, *ast.FuncLit:
		return true
	}
	return false
}

func declareAt(n ast.Node, s scopeStack) {
	switch v := n.(type) {
	case *ast.FuncLit:
		declareSignature(s, v.Type)
	case *ast.RangeStmt:
		bindRange(s, v)
	case *ast.AssignStmt:
		bindAssign(s, v)
	case *ast.ValueSpec:
		bindValueSpec(s, v)
	case *ast.CallExpr:
		bindErrorsAs(s, v)
	}
}

func declareSignature(s scopeStack, fn *ast.FuncType) {
	declareParams(s, fn.Params)
	declareParams(s, fn.Results)
}

func bindRange(s scopeStack, r *ast.RangeStmt) {
	for _, e := range []ast.Expr{r.Key, r.Value} {
		id, ok := e.(*ast.Ident)
		switch {
		case !ok:
		case r.Tok == token.DEFINE:
			s.declare(id, binding{})
		case r.Tok == token.ASSIGN:
			s.assign(id, binding{})
		}
	}
}

func declareParams(s scopeStack, params *ast.FieldList) {
	if params == nil {
		return
	}
	for _, field := range params.List {
		for _, id := range field.Names {
			s.declare(id, binding{kind: testingKind(field.Type)})
		}
	}
}

func bindAssign(s scopeStack, as *ast.AssignStmt) {
	derived := deriveBindings(s, len(as.Lhs), as.Rhs)
	for i, lhs := range as.Lhs {
		id, ok := lhs.(*ast.Ident)
		switch {
		case !ok:
		case as.Tok == token.DEFINE:
			s.declare(id, derived[i])
		default:
			s.assign(id, derived[i])
		}
	}
}

func bindValueSpec(s scopeStack, spec *ast.ValueSpec) {
	derived := deriveBindings(s, len(spec.Names), spec.Values)
	concrete := spec.Type != nil && testingKind(spec.Type) == kindConcreteTB
	for i, id := range spec.Names {
		if concrete {
			s.declare(id, binding{kind: kindConcreteTB})
			continue
		}
		s.declare(id, derived[i])
	}
}

func deriveBindings(s scopeStack, results int, rhs []ast.Expr) []binding {
	derived := make([]binding, results)
	if len(rhs) != 1 || results == 0 {
		return derived
	}
	expr := ast.Unparen(rhs[0])
	if cmd, ok := goRunCommand(expr); ok {
		derived[0] = cmd
	} else if cmd, ok := ranGoRunCommand(s, expr); ok {
		derived[results-1] = binding{kind: kindGoRunError, goFlagsGiven: cmd.goFlagsGiven}
	} else if err, ok := assertedExitError(s, expr); ok {
		derived[0] = binding{kind: kindGoRunExitError, goFlagsGiven: err.goFlagsGiven}
	} else if s.testingTB(expr) {
		derived[0] = binding{kind: kindInterfaceTB}
	}
	return derived
}
