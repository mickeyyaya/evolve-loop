//go:build acs

package cycle2662

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	inboxcmdPkg       = "./internal/cli/inboxcmd"
	isolationTestFile = "deps_isolation_test.go"
)

func goTest(t *testing.T, args ...string) string {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	full := append([]string{"test", "-count=1"}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", full...)
	if code != 0 || err != nil {
		t.Fatalf("go %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", strings.Join(full, " "), code, err, stdout, stderr)
	}
	return stdout
}

func laneForbiddenlessDepsCalls(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			callee, ok := call.Fun.(*ast.Ident)
			if !ok || (callee.Name != "Run" && callee.Name != "RunMover") {
				return true
			}
			lit, ok := call.Args[len(call.Args)-1].(*ast.CompositeLit)
			if !ok || !isDepsType(lit.Type) || literalSetsLaneForbidden(lit) {
				return true
			}
			offenders = append(offenders, filepath.Base(path)+":"+strconv.Itoa(fset.Position(call.Pos()).Line))
			return true
		})
	}
	sort.Strings(offenders)
	return offenders
}

func isDepsType(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "Deps"
}

func literalSetsLaneForbidden(lit *ast.CompositeLit) bool {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "LaneForbidden" {
			return true
		}
	}
	return false
}

func TestC2662_001_an_empty_deps_run_then_a_parallel_lint_does_not_panic_and_the_previous_host_is_restored(t *testing.T) {
	out := goTest(t, "-count=20", "-v", "-run",
		"^(TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint|TestRun_RestoresThePreviousHostOnReturn|TestRunMover_RestoresThePreviousHostOnReturn)$",
		inboxcmdPkg)
	for _, name := range []string{
		"TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint",
		"TestRun_RestoresThePreviousHostOnReturn",
		"TestRunMover_RestoresThePreviousHostOnReturn",
	} {
		if got := strings.Count(out, "--- PASS: "+name+" "); got != 20 {
			t.Errorf("%s reported %d PASS lines, want 20 (renamed, skipped or not run)", name, got)
		}
	}
}

func TestC2662_002_the_inboxcmd_package_passes_under_the_race_detector_ten_times(t *testing.T) {
	goTest(t, "-race", "-count=10", inboxcmdPkg)
}

func TestC2662_003_no_inboxcmd_test_calls_run_or_runmover_with_a_deps_literal_lacking_lane_forbidden(t *testing.T) {
	dir := filepath.Join(acsassert.GoModuleDir(t), "internal", "cli", "inboxcmd")
	var stray, isolation []string
	for _, site := range laneForbiddenlessDepsCalls(t, dir) {
		if strings.HasPrefix(site, isolationTestFile+":") {
			isolation = append(isolation, site)
			continue
		}
		stray = append(stray, site)
	}
	if len(stray) != 0 {
		t.Errorf("Run or RunMover called with a Deps literal that lacks LaneForbidden at: %s", strings.Join(stray, ", "))
	}
	if len(isolation) != 2 {
		t.Errorf("%s is the one file allowed to run Run with Deps{}, and only twice (leak test, restore test); found %v", isolationTestFile, isolation)
	}
}

func TestC2662_004_the_scanner_of_criterion_three_flags_an_empty_literal_and_accepts_a_complete_one(t *testing.T) {
	dir := t.TempDir()
	body := map[string]string{
		"bad_test.go":       "package p\nfunc f() {\n\tRun(nil, nil, nil, nil, Deps{})\n\tRunMover(nil, nil, nil, nil, Deps{QueueGate: nil})\n}\n",
		"good_test.go":      "package p\nfunc g() {\n\tRun(nil, nil, nil, nil, Deps{LaneForbidden: nil})\n\tRunMover(nil, nil, nil, nil, testDeps())\n}\n",
		"unrelated_test.go": "package p\nfunc h() {\n\tother(Deps{})\n}\n",
	}
	for name, src := range body {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(laneForbiddenlessDepsCalls(t, dir), ",")
	if want := "bad_test.go:3,bad_test.go:4"; got != want {
		t.Errorf("scanner flagged %q, want exactly %q", got, want)
	}
}

func TestC2662_005_run_runmover_and_runinboxlint_have_full_statement_coverage_with_a_green_suite(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	goTest(t, "-coverprofile="+profile, inboxcmdPkg)
	funcs, stderr, code, err := acsassert.SubprocessOutputIn(mod, "go", "tool", "cover", "-func="+profile)
	if code != 0 || err != nil {
		t.Fatalf("go tool cover exited %d (err=%v)\n%s", code, err, stderr)
	}
	for _, fn := range []string{"Run", "RunMover", "runInboxLint"} {
		pct, err := acsassert.FuncCoverage(funcs, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		if pct != 100.0 {
			t.Errorf("%s coverage = %.1f%%, want 100.0%%", fn, pct)
		}
	}
}
