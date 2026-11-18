//go:build acs

package cycle2518

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ownSetupRule = "asserts-own-setup"

const fixtureTypes = `package fx

import "testing"

type state struct{ F int }
type engine struct{ st *state }
type request struct{ State *state }

func (e *engine) run()      { e.st.F++ }
func process(r request)     {}
func use(v int)             {}
func rows() []int           { return nil }

var _ = testing.Short
`

type auditCase struct {
	name        string
	body        string
	wantFinding bool
}

func ownSetupFlagged(t *testing.T, test, body string) bool {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"lib.go":    "package fx\n",
		"types.go":  fixtureTypes,
		"x_test.go": "package fx\n\nimport \"testing\"\n\n" + body,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := testaudit.Analyze(dir, testaudit.Config{})
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	for _, f := range found {
		if f.Test == test && f.Rule == ownSetupRule {
			return true
		}
	}
	return false
}

func runCases(t *testing.T, cases []auditCase) {
	t.Helper()
	for _, c := range cases {
		if got := ownSetupFlagged(t, "TestSubject", c.body); got != c.wantFinding {
			t.Errorf("%s: %s finding = %v, want %v", c.name, ownSetupRule, got, c.wantFinding)
		}
	}
}

func TestC2518_001_FieldWrittenByCodeUnderTestIsNotOwnSetup(t *testing.T) {
	runCases(t, []auditCase{
		{name: "field set, receiver holding it runs, field asserted", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	st := &state{}
	st.F = 1
	e := engine{st: st}
	e.run()
	if st.F != 2 {
		t.Error("n")
	}
}`},
		{name: "field set, request holding it processed, field asserted", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	st := &state{}
	st.F = 1
	req := request{State: st}
	process(req)
	if st.F != 2 {
		t.Error("n")
	}
}`},
		{name: "field set, holder built twice over, then run", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	st := &state{}
	st.F = 1
	req := request{State: st}
	wrapper := &struct{ R request }{R: req}
	process(wrapper.R)
	if st.F != 2 {
		t.Error("n")
	}
}`},
		{name: "field set and asserted with no call stays a tautology", wantFinding: true, body: `
func TestSubject(t *testing.T) {
	st := &state{}
	st.F = 1
	if st.F != 1 {
		t.Error("n")
	}
}`},
		{name: "field set, holder built, no call runs, still a tautology", wantFinding: true, body: `
func TestSubject(t *testing.T) {
	st := &state{}
	st.F = 1
	e := engine{st: st}
	_ = e
	if st.F != 1 {
		t.Error("n")
	}
}`},
	})
}

func TestC2518_002_AliasOfALiteralIsStillOwnSetup(t *testing.T) {
	runCases(t, []auditCase{
		{name: "literal aliased and asserted with no call between", wantFinding: true, body: `
func TestSubject(t *testing.T) {
	v := 1
	x := v
	if x != 1 {
		t.Error("n")
	}
}`},
		{name: "alias of an alias, no call between", wantFinding: true, body: `
func TestSubject(t *testing.T) {
	v := 1
	x := v
	y := x
	if y != 1 {
		t.Error("n")
	}
}`},
		{name: "call receives the alias before the assertion", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	v := 1
	x := v
	use(x)
	if x != 1 {
		t.Error("n")
	}
}`},
		{name: "alias taken from a call result is production data", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	v := 1
	x := len(rows()) + v
	if x != 1 {
		t.Error("n")
	}
}`},
		{name: "counter bumped while ranging over production rows", wantFinding: false, body: `
func TestSubject(t *testing.T) {
	n := 0
	for range rows() {
		n++
	}
	if n != 2 {
		t.Error("n")
	}
}`},
	})
}

func funcDecl(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("%s: no func %s", path, name)
	return nil
}

func condSelectors(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		ast.Inspect(ifs.Cond, func(c ast.Node) bool {
			if sel, ok := c.(*ast.SelectorExpr); ok {
				if root, ok := sel.X.(*ast.Ident); ok {
					out[root.Name+"."+sel.Sel.Name] = true
				}
			}
			return true
		})
		return true
	})
	return out
}

func identNames(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
	return out
}

func TestC2518_003_CycleTwoFiveOhSixTestsAssertTheFieldDirectly(t *testing.T) {
	root := acsassert.RepoRoot(t)
	core := filepath.Join(root, "go", "internal", "core")
	cases := []struct {
		file, test, field string
		aliases           []string
	}{
		{"failure_learning_engine_test.go", "TestFailureLearning_LiteralOrchestratorGetsTheEngineOnce", "st.FailedAt", []string{"firstStamp", "secondStamp"}},
		{"leak_ownership_recovery_test.go", "TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate", "gate.calls", []string{"reruns"}},
	}
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "go")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
	found, err := testaudit.Analyze("internal/core", cfg)
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	for _, c := range cases {
		fn := funcDecl(t, filepath.Join(core, c.file), c.test)
		if !condSelectors(fn)[c.field] {
			t.Errorf("%s: no assertion reads %s directly", c.test, c.field)
		}
		for _, alias := range c.aliases {
			if identNames(fn)[alias] {
				t.Errorf("%s: local alias %s still hides the field", c.test, alias)
			}
		}
		for _, f := range found {
			if f.Test == c.test && f.Rule == ownSetupRule {
				t.Errorf("%s: carries a %s finding", c.test, ownSetupRule)
			}
		}
	}
}
