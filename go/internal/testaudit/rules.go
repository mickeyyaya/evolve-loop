package testaudit

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ruleNoAssertion       = "no-assertion"
	ruleLogOnly           = "log-only"
	ruleAssertsOwnSetup   = "asserts-own-setup"
	ruleBothBranchesPass  = "both-branches-pass"
	ruleSkipHidesError    = "skip-hides-error"
	ruleLiteralFieldCount = "literal-field-count"
	ruleSourcePin         = "single-file-source-pin"
	ruleASTPin            = "single-file-ast-pin"
	ruleUnverifiedFake    = "unverified-fake"
	ruleMockCallOrder     = "mock-call-order"
	ruleGoldenUpdateFlag  = "golden-update-flag"
	ruleErrorTextMatch    = "error-text-match"
	ruleSleepSync         = "sleep-sync"
)

type rule struct {
	id    string
	check func(*testCtx) bool
}

var rules = []rule{
	{ruleNoAssertion, func(c *testCtx) bool { return c.asserts == 0 && c.logs == 0 }},
	{ruleLogOnly, func(c *testCtx) bool { return c.asserts == 0 && c.logs > 0 }},
	{ruleAssertsOwnSetup, (*testCtx).assertsOwnSetup},
	{ruleBothBranchesPass, (*testCtx).bothBranchesPass},
	{ruleSkipHidesError, (*testCtx).skipHidesError},
	{ruleLiteralFieldCount, (*testCtx).literalFieldCount},
	{ruleSourcePin, func(c *testCtx) bool { return c.pinsSource(sourceReaders) }},
	{ruleASTPin, func(c *testCtx) bool { return c.pinsSource(astReaders) }},
	{ruleUnverifiedFake, (*testCtx).unverifiedFake},
	{ruleMockCallOrder, (*testCtx).mockCallOrder},
	{ruleGoldenUpdateFlag, (*testCtx).goldenUpdateFlag},
	{ruleErrorTextMatch, (*testCtx).errorTextMatch},
	{ruleSleepSync, func(c *testCtx) bool { return c.callsAny("time.Sleep") }},
}

func RuleIDs() []string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.id)
	}
	return ids
}

var sourceReaders = map[string]int{
	"os.ReadFile":                      0,
	"ioutil.ReadFile":                  0,
	"os.Open":                          0,
	"acsassert.FileContains":           1,
	"acsassert.FileNotContains":        1,
	"acsassert.FileMatchesRegex":       1,
	"acsassert.FileContainsAny":        0,
	"acsassert.FileContainsAnyChecked": 0,
	"acsassert.LineContainsAll":        0,
	"acsassert.LineContainsAllChecked": 0,
	"acsassert.CountOccurrencesAny":    0,
	"acsassert.CountInGoFunc":          0,
}

var astReaders = map[string]int{"parser.ParseFile": 1}

var failMethods = map[string]bool{"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true}

var skipMethods = map[string]bool{"Skip": true, "Skipf": true, "SkipNow": true}

var logMethods = map[string]bool{"Log": true, "Logf": true}

var callRecordNames = map[string]bool{"calls": true, "callorder": true, "invocations": true}

var stringMatchers = map[string]bool{
	"strings.Contains": true, "strings.HasPrefix": true, "strings.HasSuffix": true,
	"strings.EqualFold": true, "strings.Index": true,
}

var sequenceComparers = map[string]bool{
	"reflect.DeepEqual": true, "cmp.Diff": true, "cmp.Equal": true, "slices.Equal": true,
	"assert.Equal": true, "require.Equal": true,
}

var comparisonOps = map[token.Token]bool{
	token.EQL: true, token.NEQ: true, token.LSS: true, token.GTR: true, token.LEQ: true, token.GEQ: true,
}

type testCtx struct {
	pkg       *pkgInfo
	cfg       Config
	body      *ast.BlockStmt
	handles   map[string]bool
	vars      map[string]bool
	strConsts map[string]string
	intConsts map[string]bool
	asserts   int
	logs      int
}

func newTestCtx(pkg *pkgInfo, cfg Config, fd *ast.FuncDecl, param string) *testCtx {
	c := &testCtx{
		pkg: pkg, cfg: cfg, body: fd.Body,
		handles: map[string]bool{param: true}, vars: map[string]bool{param: true},
		strConsts: map[string]string{}, intConsts: map[string]bool{},
	}
	ast.Inspect(fd.Body, c.scan)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case c.methodOnHandle(call, logMethods):
			c.logs++
		case c.isAssertion(call):
			c.asserts++
		}
		return true
	})
	return c
}

func (c *testCtx) scan(n ast.Node) bool {
	switch x := n.(type) {
	case *ast.FuncLit:
		for _, f := range x.Type.Params.List {
			for _, id := range f.Names {
				c.vars[id.Name] = true
				if isTestingType(f.Type, "T", "B", "TB") {
					c.handles[id.Name] = true
				}
			}
		}
	case *ast.AssignStmt:
		if x.Tok == token.DEFINE {
			for _, lhs := range x.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					c.vars[id.Name] = true
				}
			}
		}
	case *ast.RangeStmt:
		if x.Tok == token.DEFINE {
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok {
					c.vars[id.Name] = true
				}
			}
		}
	case *ast.GenDecl:
		c.scanDecl(x)
	}
	return true
}

func (c *testCtx) scanDecl(gd *ast.GenDecl) {
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			lit, isLit := (*ast.BasicLit)(nil), false
			if gd.Tok == token.CONST && len(vs.Values) == len(vs.Names) {
				lit, isLit = vs.Values[i].(*ast.BasicLit)
			}
			if isLit {
				recordConst(lit, name.Name, c.strConsts, c.intConsts)
				continue
			}
			c.vars[name.Name] = true
		}
	}
}

func (c *testCtx) methodOnHandle(call *ast.CallExpr, methods map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !methods[sel.Sel.Name] {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && c.handles[id.Name]
}

func (c *testCtx) isAssertion(call *ast.CallExpr) bool {
	if c.methodOnHandle(call, failMethods) {
		return true
	}
	for _, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && c.handles[id.Name] {
			return true
		}
	}
	return false
}

func (c *testCtx) callsAny(name string) bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && qualified(call) == name {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) containsHandleCall(node ast.Node, methods map[string]bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && c.methodOnHandle(call, methods) {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) bothBranchesPass() bool {
	if c.asserts > 0 {
		return false
	}
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		if s, ok := n.(*ast.IfStmt); ok && s.Else != nil {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) skipHidesError() bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		s, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if comparesToNil(s.Cond, token.NEQ) && c.containsHandleCall(s.Body, skipMethods) {
			found = true
		}
		if comparesToNil(s.Cond, token.EQL) && s.Else != nil && c.containsHandleCall(s.Else, skipMethods) {
			found = true
		}
		return !found
	})
	return found
}

func comparesToNil(cond ast.Expr, op token.Token) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if ok && b.Op == op && (isNil(b.X) != isNil(b.Y)) {
			found = true
		}
		return !found
	})
	return found
}

func isNil(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return nil
		}
	}
}

func containsCall(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if _, ok := n.(*ast.CallExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) assertsOwnSetup() bool {
	literals := c.literalVars()
	if len(literals) == 0 {
		return false
	}
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		s, ok := n.(*ast.IfStmt)
		if !ok || !c.containsHandleCall(s.Body, failMethods) {
			return true
		}
		if pure, uses := pureOver(s.Cond, literals); pure && uses {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) literalVars() map[string]bool {
	literals := map[string]bool{}
	derived := map[string]bool{}
	links := map[string][]string{}
	ast.Inspect(c.body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.RangeStmt:
			if pure, _ := pureOver(x.X, literals); !pure {
				markDerived(derived, x.Key, x.Value)
			}
		case *ast.ForStmt:
			if init, ok := x.Init.(*ast.AssignStmt); ok {
				for _, id := range lhsIdents(init.Lhs) {
					derived[id.Name] = true
				}
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				c.recordDefinitions(lhsIdents(x.Lhs), x.Rhs, literals, derived, links)
			}
		case *ast.ValueSpec:
			c.recordDefinitions(x.Names, x.Values, literals, derived, links)
		}
		return true
	})
	c.dropEscaped(literals)
	propagateDrops(literals, links)
	return literals
}

func markDerived(derived map[string]bool, exprs ...ast.Expr) {
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok {
			derived[id.Name] = true
		}
	}
}

func lhsIdents(exprs []ast.Expr) []*ast.Ident {
	ids := make([]*ast.Ident, 0, len(exprs))
	for _, e := range exprs {
		ids = append(ids, e.(*ast.Ident))
	}
	return ids
}

func (c *testCtx) recordDefinitions(names []*ast.Ident, values []ast.Expr, literals, derived map[string]bool, links map[string][]string) {
	if len(names) != len(values) {
		return
	}
	for i, id := range names {
		if derived[id.Name] {
			continue
		}
		rhs := unparen(values[i])
		if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == token.AND {
			rhs = u.X
		}
		if ownSetupValue(rhs, literals, derived) {
			linkDefinition(id.Name, rhs, literals, links)
			literals[id.Name] = true
			continue
		}
		derived[id.Name] = true
		if _, holder := rhs.(*ast.CompositeLit); holder {
			linkDefinition(id.Name, rhs, literals, links)
			literals[id.Name] = false
		}
	}
}

func ownSetupValue(rhs ast.Expr, literals, derived map[string]bool) bool {
	if lit, ok := rhs.(*ast.CompositeLit); ok {
		return !containsCall(lit) && !mentionsAny(lit, derived)
	}
	pure, _ := pureOver(rhs, literals)
	return pure
}

func linkDefinition(name string, rhs ast.Expr, literals map[string]bool, links map[string][]string) {
	ast.Inspect(rhs, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && literals[id.Name] && id.Name != name {
			links[name] = append(links[name], id.Name)
			links[id.Name] = append(links[id.Name], name)
		}
		return true
	})
}

func propagateDrops(literals map[string]bool, links map[string][]string) {
	var pending []string
	for name := range links {
		if _, live := literals[name]; !live {
			pending = append(pending, name)
		}
	}
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, linked := range links[name] {
			if _, live := literals[linked]; live {
				delete(literals, linked)
				pending = append(pending, linked)
			}
		}
	}
}

func dropControlled(body ast.Node, literals map[string]bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				for _, e := range x.Lhs {
					if root := rootIdent(e); root != nil {
						delete(literals, root.Name)
					}
				}
			}
		case *ast.IncDecStmt:
			if root := rootIdent(x.X); root != nil {
				delete(literals, root.Name)
			}
		}
		return true
	})
}

func mentionsAny(node ast.Node, names map[string]bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) dropEscaped(literals map[string]bool) {
	ast.Inspect(c.body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if c.methodOnHandle(x, failMethods) {
				return false
			}
			dropIdents(x, literals)
			return false
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				dropIdents(x.X, literals)
			}
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				dropProductionFilled(x.Lhs, x.Rhs, literals)
				dropHandedOut(x.Lhs, x.Rhs, literals)
			}
		case *ast.IncDecStmt:
			dropProductionFilled([]ast.Expr{x.X}, nil, literals)
		case *ast.FuncLit:
			dropControlled(x.Body, literals)
		case *ast.IfStmt:
			if pure, _ := pureOver(x.Cond, literals); !pure {
				dropControlled(x.Body, literals)
				if x.Else != nil {
					dropControlled(x.Else, literals)
				}
			}
		case *ast.RangeStmt:
			if pure, _ := pureOver(x.X, literals); !pure {
				dropControlled(x.Body, literals)
			}
		case *ast.ForStmt:
			if pure, _ := pureOver(x.Cond, literals); x.Cond == nil || !pure {
				dropControlled(x.Body, literals)
			}
		}
		return true
	})
}

func dropHandedOut(lhs, rhs []ast.Expr, literals map[string]bool) {
	for i, e := range rhs {
		id, ok := unparen(e).(*ast.Ident)
		if !ok || !literals[id.Name] || i >= len(lhs) {
			continue
		}
		switch unparen(lhs[i]).(type) {
		case *ast.SelectorExpr, *ast.IndexExpr:
			if root := rootIdent(lhs[i]); root == nil || root.Name != id.Name {
				delete(literals, id.Name)
			}
		}
	}
}

func dropProductionFilled(lhs, rhs []ast.Expr, literals map[string]bool) {
	impure := false
	for _, e := range rhs {
		if pure, _ := pureOver(e, literals); !pure {
			impure = true
		}
	}
	for _, e := range lhs {
		if ix, ok := unparen(e).(*ast.IndexExpr); ok {
			if pure, _ := pureOver(ix.Index, literals); !pure {
				impure = true
			}
		}
	}
	if !impure {
		return
	}
	for _, e := range lhs {
		if root := rootIdent(e); root != nil {
			delete(literals, root.Name)
		}
	}
}

func dropIdents(node ast.Node, literals map[string]bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			delete(literals, id.Name)
		}
		return true
	})
}

func pureOver(e ast.Expr, vars map[string]bool) (pure, uses bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		return true, false
	case *ast.ParenExpr:
		return pureOver(x.X, vars)
	case *ast.UnaryExpr:
		return pureOver(x.X, vars)
	case *ast.StarExpr:
		return pureOver(x.X, vars)
	case *ast.BinaryExpr:
		lp, lu := pureOver(x.X, vars)
		rp, ru := pureOver(x.Y, vars)
		return lp && rp, lu || ru
	case *ast.Ident:
		if vars[x.Name] {
			return true, true
		}
		return x.Name == "true" || x.Name == "false" || x.Name == "nil", false
	case *ast.SelectorExpr, *ast.IndexExpr:
		if root := rootIdent(x); root != nil && vars[root.Name] {
			return true, true
		}
	}
	return false, false
}

func (c *testCtx) literalFieldCount() bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if ok && comparisonOps[b.Op] && (c.numFieldVsLiteral(b.X, b.Y) || c.numFieldVsLiteral(b.Y, b.X)) {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) numFieldVsLiteral(call, lit ast.Expr) bool {
	ce, ok := unparen(call).(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "NumField" && c.isIntLiteral(unparen(lit))
}

func (c *testCtx) isIntLiteral(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return x.Kind == token.INT
	case *ast.Ident:
		if c.intConsts[x.Name] {
			return true
		}
		return !c.vars[x.Name] && c.pkg.intConsts[x.Name]
	}
	return false
}

func (c *testCtx) constString(e ast.Expr) (string, bool) {
	switch x := unparen(e).(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(x.Value)
		return s, err == nil
	case *ast.Ident:
		if s, ok := c.strConsts[x.Name]; ok {
			return s, true
		}
		if c.vars[x.Name] {
			return "", false
		}
		s, ok := c.pkg.strConsts[x.Name]
		return s, ok
	case *ast.BinaryExpr:
		return c.joinConst(x)
	case *ast.CallExpr:
		return c.joinCall(x)
	}
	return "", false
}

func (c *testCtx) joinConst(b *ast.BinaryExpr) (string, bool) {
	if b.Op != token.ADD {
		return "", false
	}
	l, lok := c.constString(b.X)
	r, rok := c.constString(b.Y)
	return l + r, lok && rok
}

func (c *testCtx) joinCall(call *ast.CallExpr) (string, bool) {
	if !isJoin(call) {
		return "", false
	}
	parts := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		s, ok := c.constString(arg)
		if !ok {
			return "", false
		}
		parts = append(parts, s)
	}
	return filepath.Join(parts...), true
}

func isJoin(call *ast.CallExpr) bool {
	q := qualified(call)
	return q == "filepath.Join" || q == "path.Join"
}

func (c *testCtx) pathTail(e ast.Expr) (string, bool) {
	e = unparen(e)
	if s, ok := c.constString(e); ok {
		return s, true
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		if isJoin(x) && len(x.Args) > 0 {
			return c.pathTail(x.Args[len(x.Args)-1])
		}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return c.pathTail(x.Y)
		}
	}
	return "", false
}

func (c *testCtx) pinsSource(readers map[string]int) bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		idx, ok := readers[qualified(call)]
		if !ok || idx >= len(call.Args) {
			return true
		}
		if path, ok := c.constString(call.Args[idx]); ok && isSourcePath(path) {
			found = true
		}
		return !found
	})
	return found
}

func isSourcePath(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		if seg == "testdata" {
			return false
		}
	}
	return true
}

func (c *testCtx) unverifiedFake() bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || qualified(call) != "fakeclitest.Install" || len(call.Args) < 2 {
			return true
		}
		tail, ok := c.pathTail(call.Args[1])
		if !ok || !c.cfg.fakeListed(filepath.Base(tail)) {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) mockCallOrder() bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !sequenceComparers[qualified(call)] {
			return true
		}
		for _, arg := range call.Args {
			if recordsCalls(arg) {
				found = true
			}
		}
		return !found
	})
	return found
}

func recordsCalls(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.SelectorExpr:
		return callRecordNames[strings.ToLower(x.Sel.Name)]
	case *ast.Ident:
		return callRecordNames[strings.ToLower(x.Name)]
	}
	return false
}

func (c *testCtx) goldenUpdateFlag() bool {
	guarded := map[*ast.CallExpr]bool{}
	ast.Inspect(c.body, func(n ast.Node) bool {
		s, ok := n.(*ast.IfStmt)
		if !ok || !c.mentionsFlag(s.Cond) {
			return true
		}
		ast.Inspect(s.Body, func(m ast.Node) bool {
			if call, ok := m.(*ast.CallExpr); ok {
				guarded[call] = true
			}
			return true
		})
		return true
	})
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || guarded[call] || len(call.Args) == 0 {
			return true
		}
		if q := qualified(call); (q == "os.WriteFile" || q == "ioutil.WriteFile") && c.mentionsTestdata(call.Args[0]) {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) mentionsFlag(cond ast.Expr) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && c.pkg.flagVars[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

func (c *testCtx) mentionsTestdata(path ast.Expr) bool {
	found := false
	ast.Inspect(path, func(n ast.Node) bool {
		if e, ok := n.(ast.Expr); ok {
			if s, ok := c.constString(e); ok && strings.Contains(s, "testdata") {
				found = true
			}
		}
		return !found
	})
	return found
}

func (c *testCtx) errorTextMatch() bool {
	found := false
	ast.Inspect(c.body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if stringMatchers[qualified(x)] {
				for _, arg := range x.Args {
					found = found || isErrorText(arg)
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				found = found || (isErrorText(x.X) && isStringLit(x.Y)) || (isErrorText(x.Y) && isStringLit(x.X))
			}
		}
		return !found
	})
	return found
}

func isErrorText(e ast.Expr) bool {
	call, ok := unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Error"
}

func isStringLit(e ast.Expr) bool {
	lit, ok := unparen(e).(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}
