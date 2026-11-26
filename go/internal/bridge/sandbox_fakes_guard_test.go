package bridge

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testEngineConstructor = "newTestEngine"

var sandboxFakeFields = []string{"SandboxReports", "SandboxFence"}

var defaultAssertingTests = map[string]bool{
	"TestReportReadViolations_ADepsWithoutAReaderEmitsNothingAndTheDefaultsWireLogShow": true,
	"TestSandboxFence_RunsOneReportedReadUnderTheTagOfTheCell":                          true,
}

func setsBothSandboxFakes(call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	lit, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return false
	}
	if typ, ok := lit.Type.(*ast.Ident); !ok || typ.Name != "Deps" {
		return false
	}
	set := map[string]bool{}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				set[key.Name] = true
			}
		}
	}
	for _, field := range sandboxFakeFields {
		if !set[field] {
			return false
		}
	}
	return true
}

func realSandboxEngines(fset *token.FileSet, f *ast.File) []string {
	var hits []string
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && (fn.Name.Name == testEngineConstructor || defaultAssertingTests[fn.Name.Name]) {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && calleeName(call) == "NewEngine" && !setsBothSandboxFakes(call) {
				hits = append(hits, fset.Position(call.Pos()).String())
			}
			return true
		})
	}
	return hits
}

func assignedFields(fn *ast.FuncDecl) map[string]bool {
	assigned := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range as.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					assigned[sel.Sel.Name] = true
				}
			}
		}
		return true
	})
	return assigned
}

func testEngineConstructorGaps(files []*ast.File) []string {
	var gaps []string
	found := 0
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != testEngineConstructor || fn.Recv != nil {
				continue
			}
			found++
			assigned := assignedFields(fn)
			for _, field := range sandboxFakeFields {
				if !assigned[field] {
					gaps = append(gaps, testEngineConstructor+" never assigns Deps."+field)
				}
			}
		}
	}
	if found != 1 {
		gaps = append(gaps, testEngineConstructor+" is not declared exactly once in the test files")
	}
	return gaps
}

func TestBridgeTestEnginesInjectSandboxFakes(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var hits []string
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		hits = append(hits, realSandboxEngines(fset, f)...)
	}
	if len(files) == 0 {
		t.Fatal("the guard found no test file, so it checks nothing; fix the scan")
	}
	if gaps := testEngineConstructorGaps(files); len(gaps) > 0 {
		t.Errorf("the test engine constructor does not inject the sandbox fakes:\n%s", strings.Join(gaps, "\n"))
	}
	if len(hits) > 0 {
		t.Errorf("a test builds an Engine with NewEngine and no SandboxReports and SandboxFence, so on macOS it reaches the real log show poll (60 s fence deadline); build it with newTestEngine:\n%s", strings.Join(hits, "\n"))
	}
}

func TestRealSandboxEngines_FlagsADirectNewEngineWithoutBothFakes(t *testing.T) {
	t.Parallel()
	const body = `
package bridge

func TestBare(t *testing.T) {
	NewEngine(Deps{Tmux: tmux})
	NewEngine(deps)
	NewEngine(Deps{SandboxReports: r})
	NewEngine(Deps{SandboxReports: r, SandboxFence: f})
	newTestEngine(Deps{})
}

func TestReportReadViolations_ADepsWithoutAReaderEmitsNothingAndTheDefaultsWireLogShow(t *testing.T) {
	NewEngine(Deps{})
}

func newTestEngine(d Deps) *Engine {
	return NewEngine(d)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x_test.go", body, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if hits := realSandboxEngines(fset, f); len(hits) != 3 {
		t.Fatalf("hits = %v, want the three bare NewEngine calls of TestBare and no other", hits)
	}
	tagged, err := parser.ParseFile(fset, "y_test.go", "//go:build integration\n\npackage bridge\n\nfunc TestIt() { NewEngine(Deps{}) }\n", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if hits := realSandboxEngines(fset, tagged); len(hits) != 1 {
		t.Fatalf("an integration-tagged file gets no exemption, hits = %v, want 1", hits)
	}
	if gaps := testEngineConstructorGaps([]*ast.File{f}); len(gaps) != 2 {
		t.Fatalf("gaps = %v, want one per missing fake", gaps)
	}
}

func TestNewTestEngine_SandboxReadsNeverSpawnABinary(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	sleeps := 0
	eng := newTestEngine(Deps{
		Now:   func() time.Time { return now },
		Sleep: func(time.Duration) { sleeps++; now = now.Add(time.Hour) },
	})
	_, gap, err := eng.deps.cellReports(context.Background(), "norun/0/agent/p1nyt2d709b9633f4", now)
	if err != nil {
		t.Fatalf("cellReports: %v", err)
	}
	if gap != (readGap{}) || sleeps != 0 {
		t.Fatalf("gap = %+v after %d polls, want a fenced answer on the first read: the test engine reached the real log show poll", gap, sleeps)
	}
}

func TestNewTestEngine_SandboxFakesAnswerWithoutStartingAProcess(t *testing.T) {
	t.Parallel()
	eng := newTestEngine(Deps{})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	const tag = "norun/0/agent/p1nyt2d709b9633f4"
	if err := eng.deps.SandboxFence(cancelled, tag); err != nil {
		t.Fatalf("the fence under a cancelled context = %v, want nil: a real sandbox-exec runner returns the context error before it starts", err)
	}
	raw, err := eng.deps.SandboxReports(cancelled, tag, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("the report reader under a cancelled context = %v, want nil: a real log show runner returns the context error before it starts", err)
	}
	if !strings.Contains(string(raw), fenceMessage(tag)) {
		t.Fatalf("report = %q, want the fence line of the tag", raw)
	}
}
