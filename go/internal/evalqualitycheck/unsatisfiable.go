package evalqualitycheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	UnsatisfiableKindInvertedIdiom  = "inverted-idiom"
	UnsatisfiableKindAbsenceMessage = "absence-message"
	UnsatisfiableKindGoRunExitCode  = "go-run-exit-code"
	acsassertPkgName                = "acsassert"
)

type UnsatisfiableFinding struct {
	Func   string
	File   string
	Kind   string
	Reason string
}

type UnsatisfiableLintReport struct {
	Files    []string
	Findings []UnsatisfiableFinding
}

func (r UnsatisfiableLintReport) Linted() int { return len(r.Files) }

var positiveAssertions = map[string]string{
	"FileContains":     "acsassert.FileNotContains",
	"FileMatchesRegex": "acsassert.FileNotContains",
	"FileExists":       "a swallowing-TB probe or os.Stat with errors.Is(err, fs.ErrNotExist)",
	"JSONFieldEquals":  "a swallowing-TB probe",
}

var concreteTestingTypes = map[string]bool{"T": true, "B": true, "F": true}

var failureReporters = map[string]bool{
	"Errorf": true, "Error": true, "Fatalf": true, "Fatal": true, "Fail": true, "FailNow": true,
}

var absenceWordRE = regexp.MustCompile(`(?i)\b(still|remains?|remaining|lingers?|leftover|duplicated|retained|present)\b`)

var missingWordRE = regexp.MustCompile(`(?i)\b(not|no|missing|absent|without|lacks?|never|undocumented|unwired|must|should|expected|expect)\b|n't`)

func LintUnsatisfiablePredicates(path string) (UnsatisfiableLintReport, error) {
	report := UnsatisfiableLintReport{}
	paths, err := predicateSourcePaths(path)
	if err != nil {
		return report, err
	}
	fset := token.NewFileSet()
	for _, p := range paths {
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return report, perr
		}
		name := filepath.Base(p)
		report.Files = append(report.Files, name)
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				report.Findings = append(report.Findings, lintFuncUnsatisfiable(fn, name)...)
			}
		}
	}
	return report, nil
}

type verdict struct {
	kind   string
	reason string
}

var unsatisfiableJudges = []func(ast.Node, scopeStack) (verdict, bool){assertionVerdict, goRunExitCodeVerdict}

func lintFuncUnsatisfiable(fn *ast.FuncDecl, file string) []UnsatisfiableFinding {
	var out []UnsatisfiableFinding
	walkScoped(fn, func(n ast.Node, scopes scopeStack) {
		for _, judge := range unsatisfiableJudges {
			if v, found := judge(n, scopes); found {
				out = append(out, UnsatisfiableFinding{Func: fn.Name.Name, File: file, Kind: v.kind, Reason: v.reason})
			}
		}
	})
	return out
}

func assertionVerdict(n ast.Node, scopes scopeStack) (verdict, bool) {
	ifs, ok := n.(*ast.IfStmt)
	if !ok {
		return verdict{}, false
	}
	negated, call := positiveAssertionCond(ifs.Cond)
	if call == nil {
		return verdict{}, false
	}
	prim := call.Fun.(*ast.SelectorExpr).Sel.Name
	msgs, reports := failureReports(ifs.Body)
	switch {
	case !negated && reports && len(call.Args) > 0 && scopes.testingTB(call.Args[0]):
		return verdict{UnsatisfiableKindInvertedIdiom, fmt.Sprintf("%s used as a bare failure condition fails on the absent state and again on the present one; use %s for absence", prim, positiveAssertions[prim])}, true
	case negated && absenceIntent(msgs):
		return verdict{UnsatisfiableKindAbsenceMessage, fmt.Sprintf("%s is a positive assertion but its failure text demands absence; use %s", prim, positiveAssertions[prim])}, true
	}
	return verdict{}, false
}

func testingKind(declared ast.Expr) nameKind {
	if star, ok := declared.(*ast.StarExpr); ok {
		declared = star.X
	}
	sel, ok := declared.(*ast.SelectorExpr)
	if !ok {
		return kindOther
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "testing" {
		return kindOther
	}
	switch {
	case concreteTestingTypes[sel.Sel.Name]:
		return kindConcreteTB
	case sel.Sel.Name == "TB":
		return kindInterfaceTB
	}
	return kindOther
}

func positiveAssertionCond(cond ast.Expr) (negated bool, call *ast.CallExpr) {
	expr := ast.Unparen(cond)
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		negated = true
		expr = ast.Unparen(u.X)
	}
	c, ok := expr.(*ast.CallExpr)
	if !ok {
		return false, nil
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false, nil
	}
	if _, known := positiveAssertions[sel.Sel.Name]; !known {
		return false, nil
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != acsassertPkgName {
		return false, nil
	}
	return negated, c
}

func failureReports(body *ast.BlockStmt) (messages []string, reports bool) {
	for _, stmt := range body.List {
		call := failureReporterCall(stmt)
		if call == nil {
			continue
		}
		reports = true
		var parts []string
		for _, arg := range call.Args {
			parts = append(parts, stringLiterals(arg)...)
		}
		if len(parts) > 0 {
			messages = append(messages, strings.Join(parts, ""))
		}
	}
	return messages, reports
}

func failureReporterCall(stmt ast.Stmt) *ast.CallExpr {
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok && failureReporters[sel.Sel.Name] {
		return c
	}
	return nil
}

func stringLiterals(expr ast.Expr) []string {
	var out []string
	ast.Inspect(expr, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, s)
			}
		}
		return true
	})
	return out
}

func absenceIntent(messages []string) bool {
	for _, m := range messages {
		if absenceWordRE.MatchString(m) && !missingWordRE.MatchString(m) {
			return true
		}
	}
	return false
}
