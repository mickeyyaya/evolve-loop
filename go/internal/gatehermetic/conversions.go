package gatehermetic

import (
	"go/ast"
	"go/token"
	"go/types"
)

func (s *scanner) convert(e ast.Expr, target types.Type) {
	if e == nil || target == nil {
		return
	}
	if s.convertTuple(e, func(int) types.Type { return target }) {
		return
	}
	s.flow(e, target)
	if t := s.info.TypeOf(e); t != nil {
		s.convertType(e.Pos(), t, target)
	}
}

func (s *scanner) convertType(pos token.Pos, t, target types.Type) {
	if target == nil || !types.IsInterface(target) {
		return
	}
	s.out.types = append(s.out.types, t)
	if s.isRunnerValue(t) {
		s.add(classExec, pos)
	}
}

func (s *scanner) convertTuple(e ast.Expr, target func(i int) types.Type) bool {
	tup, ok := s.info.TypeOf(e).(*types.Tuple)
	for i := 0; ok && i < tup.Len(); i++ {
		s.convertType(e.Pos(), tup.At(i).Type(), target(i))
	}
	return ok
}

func (s *scanner) flow(e ast.Expr, target types.Type) {
	n, ok := types.Unalias(target).(*types.Named)
	r := s.runnerOf()
	if !ok || r.sig == nil || n.Obj().Pkg() == nil || n.Obj().Pkg().Path() != r.entry.Package || n.Obj().Name() != r.entry.Name {
		return
	}
	if id := calleeIdent(e); id != nil {
		if s.flows == nil {
			s.flows = map[*ast.Ident]bool{}
		}
		s.flows[id] = true
	}
}

func (s *scanner) conversions(n ast.Node) {
	if t, ok := s.g.targets[n]; ok {
		s.convert(n.(ast.Expr), t)
	}
	switch x := n.(type) {
	case *ast.CallExpr:
		s.callConversions(x)
	case *ast.CompositeLit:
		s.litConversions(x)
	case *ast.AssignStmt:
		s.assignConversions(x)
	case *ast.IndexExpr:
		if m, ok := s.info.TypeOf(x.X).Underlying().(*types.Map); ok {
			s.convert(x.Index, m.Key())
		}
	case *ast.ReturnStmt:
		s.returnConversions(x)
	case *ast.SendStmt:
		if ch, ok := s.info.TypeOf(x.Chan).Underlying().(*types.Chan); ok {
			s.convert(x.Value, ch.Elem())
		}
	case *ast.ValueSpec:
		if x.Type != nil {
			for _, v := range x.Values {
				s.convert(v, s.info.TypeOf(x.Type))
			}
		}
	}
}

func (s *scanner) assignConversions(a *ast.AssignStmt) {
	if len(a.Lhs) == len(a.Rhs) {
		for i, l := range a.Lhs {
			s.convert(a.Rhs[i], s.info.TypeOf(l))
		}
		return
	}
	s.convertTuple(a.Rhs[0], func(i int) types.Type { return s.info.TypeOf(a.Lhs[i]) })
}

func (s *scanner) callConversions(c *ast.CallExpr) {
	tv := s.info.Types[c.Fun]
	if tv.IsType() {
		s.convert(c.Args[0], tv.Type)
		return
	}
	if sig, ok := tv.Type.Underlying().(*types.Signature); ok {
		s.argConversions(c, sig)
	}
}

func (s *scanner) argConversions(c *ast.CallExpr, sig *types.Signature) {
	spread := c.Ellipsis.IsValid()
	if len(c.Args) == 1 && s.convertTuple(c.Args[0], func(i int) types.Type { return paramType(sig, i, spread) }) {
		return
	}
	for i, a := range c.Args {
		s.convert(a, paramType(sig, i, spread))
	}
}

func paramType(sig *types.Signature, i int, spread bool) types.Type {
	n := sig.Params().Len()
	if !sig.Variadic() || i < n-1 {
		return sig.Params().At(i).Type()
	}
	last := sig.Params().At(n - 1).Type()
	if sl, ok := last.(*types.Slice); ok && !spread {
		return sl.Elem()
	}
	return last
}

func (s *scanner) litConversions(x *ast.CompositeLit) {
	u := s.info.TypeOf(x).Underlying()
	if p, ok := u.(*types.Pointer); ok {
		u = p.Elem().Underlying()
	}
	for i, el := range x.Elts {
		key, val := ast.Expr(nil), el
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			key, val = kv.Key, kv.Value
		}
		switch c := u.(type) {
		case *types.Struct:
			s.convert(val, s.fieldType(c, i, key))
		case *types.Slice:
			s.convert(val, c.Elem())
		case *types.Array:
			s.convert(val, c.Elem())
		case *types.Map:
			s.convert(key, c.Key())
			s.convert(val, c.Elem())
		}
	}
}

func (s *scanner) fieldType(st *types.Struct, i int, key ast.Expr) types.Type {
	if id, ok := key.(*ast.Ident); ok {
		return s.info.Uses[id].Type()
	}
	return st.Field(i).Type()
}

func (s *scanner) returnConversions(r *ast.ReturnStmt) {
	sig := s.sig
	for i := len(s.stack) - 1; i >= 0; i-- {
		if lit, ok := s.stack[i].(*ast.FuncLit); ok {
			sig, _ = s.info.TypeOf(lit).(*types.Signature)
			break
		}
	}
	if sig == nil {
		return
	}
	if len(r.Results) == 1 && s.convertTuple(r.Results[0], func(i int) types.Type { return sig.Results().At(i).Type() }) || sig.Results().Len() != len(r.Results) {
		return
	}
	for i, e := range r.Results {
		s.convert(e, sig.Results().At(i).Type())
	}
}
