//go:build acs

package cycle1721

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	poolPkg      = "./cmd/evolve"
	poolPkgRel   = "go/cmd/evolve"
	poolTestRel  = "go/cmd/evolve/cmd_loop_pool_test.go"
	backfillTest = "TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning"

	slowHandoffSeconds = 4
	slowHandoffMarker  = "acs-c1721-slow-handoff-injected"
	raceRuns           = 50
)

var poolFileTests = []string{
	"TestShouldRunPool_GateTable",
	"TestShouldRunWaveAndPool_MutuallyExclusive",
	backfillTest,
	"TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch",
	"TestDispatchPoolIteration_WaveConfigInertNoLaunch",
	"TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches",
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		t.Cleanup(cancel)
	}
	return ctx
}

func runFromModuleRoot(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func parsePoolTestFile(t *testing.T) (*token.FileSet, *ast.File, []byte, string) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), poolTestRel)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("RED: read %s: %v", poolTestRel, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", poolTestRel, err)
	}
	return fset, file, src, path
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func launchBodyOffset(fset *token.FileSet, fn *ast.FuncDecl) (int, error) {
	var launchArg ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if launchArg != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "dispatchPoolIteration" && len(call.Args) >= 5 {
			launchArg = call.Args[4]
		}
		return true
	})
	if launchArg == nil {
		return 0, fmt.Errorf("%s no longer calls dispatchPoolIteration(ctx, fc, preflight, planFn, launch, waveIndex)", fn.Name.Name)
	}
	lit, ok := launchArg.(*ast.FuncLit)
	if !ok {
		id, isIdent := launchArg.(*ast.Ident)
		if !isIdent {
			return 0, fmt.Errorf("launch argument is %T, want a func literal or a local bound to one", launchArg)
		}
		lit = localFuncLit(fn, id.Name)
	}
	if lit == nil {
		return 0, fmt.Errorf("launch callback of %s is not a func literal in the test body — the slow-handoff injection seam is gone", fn.Name.Name)
	}
	return fset.Position(lit.Body.Lbrace).Offset + 1, nil
}

func localFuncLit(fn *ast.FuncDecl, name string) *ast.FuncLit {
	var found *ast.FuncLit
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name && i < len(s.Rhs) {
					if lit, ok := s.Rhs[i].(*ast.FuncLit); ok {
						found = lit
					}
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if id.Name == name && i < len(s.Values) {
					if lit, ok := s.Values[i].(*ast.FuncLit); ok {
						found = lit
					}
				}
			}
		}
		return true
	})
	return found
}

func TestC1721_001_BackfillTestToleratesSlowLaneHandoff(t *testing.T) {
	fset, file, src, path := parsePoolTestFile(t)
	fn := findFunc(file, backfillTest)
	if fn == nil {
		t.Fatalf("RED: %s is missing from %s", backfillTest, poolTestRel)
	}
	off, err := launchBodyOffset(fset, fn)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	mutated := string(src[:off]) + "\n\tacsC1721SlowHandoff()\n" + string(src[off:])
	if _, err := parser.ParseFile(token.NewFileSet(), path, mutated, 0); err != nil {
		t.Fatalf("harness: slow-handoff mutation does not parse: %v", err)
	}
	helper := fmt.Sprintf(`package %s

import (
	"fmt"
	"time"
)

func acsC1721SlowHandoff() {
	fmt.Println(%q)
	time.Sleep(%d * time.Second)
}
`, file.Name.Name, slowHandoffMarker, slowHandoffSeconds)

	dir := t.TempDir()
	mutatedPath := filepath.Join(dir, "cmd_loop_pool_test.go")
	helperPath := filepath.Join(dir, "acs_c1721_slow_handoff_test.go")
	if err := os.WriteFile(mutatedPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helperPath, []byte(helper), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(acsassert.RepoRoot(t), poolPkgRel)
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{
		path: mutatedPath,
		filepath.Join(pkgDir, "acs_c1721_slow_handoff_test.go"): helperPath,
	}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatal(err)
	}

	out := runFromModuleRoot(t, exec.CommandContext(testContext(t),
		"go", "test", "-v", "-race", "-count=1", "-overlay="+overlayPath, "-run", "^"+backfillTest+"$", poolPkg))
	if !strings.Contains(out, slowHandoffMarker) {
		t.Fatalf("harness: the slow-handoff injection never ran (marker absent), so this run proves nothing:\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: "+backfillTest+" ") || strings.Contains(out, "--- FAIL:") {
		t.Errorf("RED: %s fails when a pool lane takes %ds to start — its correctness still rests on a wall-clock bet:\n%s",
			backfillTest, slowHandoffSeconds, out)
	}
}

func TestC1721_002_PoolTestsPassFiftyRaceRuns(t *testing.T) {
	_, file, _, _ := parsePoolTestFile(t)
	var names []string
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
			names = append(names, fn.Name.Name)
		}
	}
	for _, want := range poolFileTests {
		if !containsString(names, want) {
			t.Fatalf("RED: %s no longer declares %s", poolTestRel, want)
		}
	}

	out := runFromModuleRoot(t, exec.CommandContext(testContext(t),
		"go", "test", "-v", "-race", fmt.Sprintf("-count=%d", raceRuns), "-run", "^("+strings.Join(names, "|")+")$", poolPkg))
	for _, n := range names {
		if got := strings.Count(out, "--- PASS: "+n+" "); got != raceRuns {
			t.Errorf("RED: %s passed %d/%d runs under -race", n, got, raceRuns)
		}
	}
	if strings.Contains(out, "--- FAIL:") || t.Failed() {
		t.Errorf("output:\n%s", out)
	}
}

var timeBudgetArg = map[string]int{
	"time.After":          0,
	"time.AfterFunc":      0,
	"time.NewTimer":       0,
	"time.NewTicker":      0,
	"time.Sleep":          0,
	"time.Tick":           0,
	"context.WithTimeout": 1,
}

type budgetChecker struct {
	fset      *token.FileSet
	commented map[string]bool
	funcs     map[string]bool
}

func newBudgetChecker(fset *token.FileSet, files []*ast.File) budgetChecker {
	c := budgetChecker{fset: fset, commented: map[string]bool{}, funcs: map[string]bool{}}
	for _, f := range files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					c.commented[decl.Name.Name] = hasText(decl.Doc)
					c.funcs[decl.Name.Name] = true
				}
			case *ast.GenDecl:
				if decl.Tok != token.CONST && decl.Tok != token.VAR {
					continue
				}
				for _, spec := range decl.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, id := range vs.Names {
						c.commented[id.Name] = hasText(vs.Doc) || hasText(vs.Comment) || hasText(decl.Doc)
					}
				}
			}
		}
	}
	return c
}

func hasText(cg *ast.CommentGroup) bool {
	return cg != nil && strings.TrimSpace(cg.Text()) != ""
}

func (c budgetChecker) violations(fn *ast.FuncDecl) (sites int, out []string) {
	locals := map[string]ast.Expr{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if s, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					if len(s.Lhs) == len(s.Rhs) {
						locals[id.Name] = s.Rhs[i]
					} else if len(s.Rhs) == 1 {
						locals[id.Name] = s.Rhs[0]
					}
				}
			}
		}
		return true
	})
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && c.funcs[id.Name] {
			for _, arg := range call.Args {
				if isBareDuration(arg) {
					out = append(out, fmt.Sprintf("%s:%d %s: bare literal duration passed as a budget", fn.Name.Name, c.fset.Position(arg.Pos()).Line, id.Name))
				}
			}
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		idx, isBudget := timeBudgetArg[pkg.Name+"."+sel.Sel.Name]
		if !isBudget || idx >= len(call.Args) {
			return true
		}
		sites++
		if reason := c.judge(call.Args[idx], locals, 0); reason != "" {
			out = append(out, fmt.Sprintf("%s:%d %s.%s: %s", fn.Name.Name, c.fset.Position(call.Pos()).Line, pkg.Name, sel.Sel.Name, reason))
		}
		return true
	})
	return sites, out
}

func (c budgetChecker) judge(expr ast.Expr, locals map[string]ast.Expr, depth int) string {
	derived := false
	var problems []string
	ast.Inspect(expr, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "time" {
				return false
			}
			derived = true
			return false
		case *ast.Ident:
			if commented, isPkg := c.commented[x.Name]; isPkg {
				if commented {
					derived = true
				} else {
					problems = append(problems, fmt.Sprintf("names %s, whose declaration carries no comment explaining the margin", x.Name))
				}
				return false
			}
			if rhs, isLocal := locals[x.Name]; isLocal && depth < 4 {
				if r := c.judge(rhs, locals, depth+1); r != "" {
					problems = append(problems, fmt.Sprintf("local %s: %s", x.Name, r))
				} else {
					derived = true
				}
				return false
			}
			derived = true
			return false
		}
		return true
	})
	switch {
	case len(problems) > 0:
		return strings.Join(problems, "; ")
	case !derived:
		return "bare literal duration"
	}
	return ""
}

var timeUnits = map[string]bool{
	"Nanosecond": true, "Microsecond": true, "Millisecond": true,
	"Second": true, "Minute": true, "Hour": true,
}

func isBareDuration(expr ast.Expr) bool {
	unit, bare := false, true
	ast.Inspect(expr, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "time" && timeUnits[x.Sel.Name] {
				unit = true
			} else {
				bare = false
			}
			return false
		case *ast.Ident, *ast.CallExpr, *ast.FuncLit, *ast.CompositeLit:
			bare = false
			return false
		}
		return true
	})
	return unit && bare
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

const checkerFixture = `package main

import (
	"context"
	"testing"
	"time"
)

// documented is sized for -race on a loaded shared runner.
const documented = 10 * time.Second

const undocumented = 10 * time.Second

func TestBare(t *testing.T)        { <-time.After(2 * time.Second) }
func TestUndocumented(t *testing.T) { <-time.After(undocumented) }
func TestLocalBare(t *testing.T)    { d := 3 * time.Second; <-time.After(d) }
func TestCtxBare(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	<-ctx.Done()
}
func TestDocumented(t *testing.T)   { <-time.After(3 * documented) }
func TestLocalDocumented(t *testing.T) { d := documented; <-time.After(d) }
func TestDeadline(t *testing.T)     { d, _ := t.Deadline(); <-time.After(time.Until(d)) }

// wait blocks on ch for at most d.
func wait(ch chan int, d time.Duration) {
	select {
	case <-ch:
	case <-time.After(d):
	}
}
func TestHelperBare(t *testing.T)       { wait(nil, 2*time.Second) }
func TestHelperDocumented(t *testing.T) { wait(nil, documented) }
`

func requireCheckerSound(t *testing.T) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", checkerFixture, parser.ParseComments)
	if err != nil {
		t.Fatalf("harness: fixture: %v", err)
	}
	c := newBudgetChecker(fset, []*ast.File{f})
	want := map[string]int{
		"TestBare": 1, "TestUndocumented": 1, "TestLocalBare": 1, "TestCtxBare": 1,
		"TestDocumented": 0, "TestLocalDocumented": 0, "TestDeadline": 0,
		"wait": 0, "TestHelperBare": 1, "TestHelperDocumented": 0,
	}
	for name, n := range want {
		fn := findFunc(f, name)
		if fn == nil {
			t.Fatalf("harness: fixture lacks %s", name)
		}
		if _, got := c.violations(fn); len(got) != n {
			t.Fatalf("harness: budget inspector unsound on %s: got %d violation(s) %v, want %d", name, len(got), got, n)
		}
	}
}

func poolPackageChecker(t *testing.T, fset *token.FileSet, file *ast.File) budgetChecker {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), poolPkgRel)
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("harness: list %s: %v", poolPkgRel, err)
	}
	files := []*ast.File{file}
	for _, p := range paths {
		if filepath.Base(p) == filepath.Base(poolTestRel) {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("harness: parse %s: %v", p, err)
		}
		if f.Name.Name == file.Name.Name {
			files = append(files, f)
		}
	}
	return newBudgetChecker(fset, files)
}

// acs-predicate: config-check — AC2 is by its own words a property of the test
func TestC1721_003_BackfillBudgetIsNamedAndCommented(t *testing.T) {
	requireCheckerSound(t)
	fset, file, _, _ := parsePoolTestFile(t)
	fn := findFunc(file, backfillTest)
	if fn == nil {
		t.Fatalf("RED: %s is missing from %s", backfillTest, poolTestRel)
	}
	sites, bad := poolPackageChecker(t, fset, file).violations(fn)
	t.Logf("%s holds %d wall-clock budget site(s)", backfillTest, sites)
	for _, v := range bad {
		t.Errorf("RED: %s", v)
	}
}

// acs-predicate: config-check — AC3 asks whether any sibling in the file shares
func TestC1721_004_NoTestInPoolFileHoldsBareWallClockBudget(t *testing.T) {
	requireCheckerSound(t)
	fset, file, _, _ := parsePoolTestFile(t)
	c := poolPackageChecker(t, fset, file)
	var bad []string
	for _, name := range poolFileTests {
		if findFunc(file, name) == nil {
			t.Errorf("RED: %s no longer declares %s — a sibling is ruled out by fixing it, not by deleting it", poolTestRel, name)
		}
	}
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			_, v := c.violations(fn)
			bad = append(bad, v...)
		}
	}
	sort.Strings(bad)
	for _, v := range bad {
		t.Errorf("RED: %s", v)
	}
}
