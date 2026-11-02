//go:build acs

package cycle2103

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pkgRel           = "internal/practicewatch"
	minTruncationDur = 30 * time.Second
)

var unitDurations = map[string]time.Duration{
	"Nanosecond":  time.Nanosecond,
	"Microsecond": time.Microsecond,
	"Millisecond": time.Millisecond,
	"Second":      time.Second,
	"Minute":      time.Minute,
	"Hour":        time.Hour,
}

type testFuncs map[string]*ast.FuncDecl

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func loadTestFuncs(t *testing.T) testFuncs {
	t.Helper()
	dir := filepath.Join(goDir(t), pkgRel)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	funcs := testFuncs{}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			if !strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
					funcs[fn.Name.Name] = fn
				}
			}
		}
	}
	return funcs
}

func evalDuration(e ast.Expr) (time.Duration, error) {
	switch v := e.(type) {
	case *ast.ParenExpr:
		return evalDuration(v.X)
	case *ast.BasicLit:
		n, err := strconv.ParseInt(v.Value, 0, 64)
		if err != nil {
			return 0, fmt.Errorf("literal %s: %w", v.Value, err)
		}
		return time.Duration(n), nil
	case *ast.SelectorExpr:
		pkg, ok := v.X.(*ast.Ident)
		if !ok || pkg.Name != "time" {
			return 0, fmt.Errorf("selector %v is not a time unit", v.Sel.Name)
		}
		unit, known := unitDurations[v.Sel.Name]
		if !known {
			return 0, fmt.Errorf("time.%s is not a duration unit", v.Sel.Name)
		}
		return unit, nil
	case *ast.BinaryExpr:
		if v.Op != token.MUL {
			return 0, fmt.Errorf("operator %s is not supported", v.Op)
		}
		l, err := evalDuration(v.X)
		if err != nil {
			return 0, err
		}
		r, err := evalDuration(v.Y)
		if err != nil {
			return 0, err
		}
		return l * r, nil
	}
	return 0, fmt.Errorf("expression %T is not a constant duration", e)
}

func timeoutAssignments(fn *ast.FuncDecl) []ast.Expr {
	var rhs []ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Timeout" && i < len(v.Rhs) {
					rhs = append(rhs, v.Rhs[i])
				}
			}
		case *ast.KeyValueExpr:
			if key, ok := v.Key.(*ast.Ident); ok && key.Name == "Timeout" {
				rhs = append(rhs, v.Value)
			}
		}
		return true
	})
	return rhs
}

func (f testFuncs) effectiveTimeout(t *testing.T, name string) time.Duration {
	t.Helper()
	fn, ok := f[name]
	if !ok {
		t.Fatalf("test function %s not found in %s", name, pkgRel)
	}
	exprs := timeoutAssignments(fn)
	if len(exprs) == 0 {
		helper, found := f["optionsFor"]
		if !found {
			t.Fatalf("%s sets no Timeout and optionsFor is missing", name)
		}
		exprs = timeoutAssignments(helper)
	}
	if len(exprs) == 0 {
		t.Fatalf("%s: no Timeout is set by the test or by optionsFor", name)
	}
	d, err := evalDuration(exprs[len(exprs)-1])
	if err != nil {
		t.Fatalf("%s: cannot evaluate Timeout: %v", name, err)
	}
	return d
}

func callsAll(fn *ast.FuncDecl, callees ...string) bool {
	seen := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, isIdent := call.Fun.(*ast.Ident); isIdent {
				seen[id.Name] = true
			}
		}
		return true
	})
	for _, c := range callees {
		if !seen[c] {
			return false
		}
	}
	return true
}

func (f testFuncs) names(keep func(string, *ast.FuncDecl) bool) []string {
	var out []string
	for name, fn := range f {
		if strings.HasPrefix(name, "Test") && keep(name, fn) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (f testFuncs) truncationWatchTests() []string {
	return f.names(func(_ string, fn *ast.FuncDecl) bool {
		return callsAll(fn, "oversizedServer", "Watch")
	})
}

func hasDeadlineText(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(strings.ToLower(lit.Value), "deadline") {
			found = true
		}
		return !found
	})
	return found
}

func mentionsIdent(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func (f testFuncs) deadlineTests() []string {
	return f.names(func(_ string, fn *ast.FuncDecl) bool {
		return callsAll(fn, "Watch") && mentionsIdent(fn, "StatusError") && hasDeadlineText(fn)
	})
}

func runPattern(names []string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

func TestC2103_001_TruncationWatchTestsSetATimeoutOfAtLeast30Seconds(t *testing.T) {
	funcs := loadTestFuncs(t)
	names := funcs.truncationWatchTests()
	if len(names) == 0 {
		t.Fatalf("RED: no test in %s drives Watch against oversizedServer", pkgRel)
	}
	for _, name := range names {
		if got := funcs.effectiveTimeout(t, name); got < minTruncationDur {
			t.Errorf("RED: %s runs Watch with Timeout %s, want at least %s", name, got, minTruncationDur)
		}
	}
}

func TestC2103_002_ASlowSourceStillEndsInStatusErrorWithTheDeadlineText(t *testing.T) {
	funcs := loadTestFuncs(t)
	names := funcs.deadlineTests()
	if len(names) == 0 {
		t.Fatalf("RED: no test in %s asserts that a slow source ends in StatusError with the deadline text", pkgRel)
	}
	for _, name := range names {
		if got := funcs.effectiveTimeout(t, name); got >= minTruncationDur {
			t.Errorf("RED: %s has Timeout %s, so a slow source cannot reach its deadline in test time", name, got)
		}
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-race", "-count=1", "-run", runPattern(names), "./"+pkgRel)
	if err != nil || code != 0 {
		t.Fatalf("RED: %v failed (exit=%d, err=%v):\n%s%s", names, code, err, acsassert.OutputTail(stdout, 30), acsassert.OutputTail(stderr, 30))
	}
	if strings.Contains(stdout, "no tests to run") {
		t.Errorf("RED: the pattern %s matched no test", runPattern(names))
	}
}
