//go:build acs

package cycle1834

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const gittestImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/gittest"

type attempt struct {
	out []byte
	err error
}

type scriptedRunner struct {
	attempts []attempt
	calls    int
}

func (s *scriptedRunner) run() ([]byte, error) {
	s.calls++
	if s.calls > len(s.attempts) {
		return []byte("unscripted extra attempt"), nil
	}
	a := s.attempts[s.calls-1]
	return a.out, a.err
}

func forkExecGitError(errno syscall.Errno) error {
	return &os.PathError{Op: "fork/exec", Path: "/usr/bin/git", Err: errno}
}

func goModuleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = goModuleRoot(t)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func requireEveryTestPassed(t *testing.T, out string, names []string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, out)
		}
	}
}

func TestC1834_001_CaptureWithEBADFRetryAbsorbsOneTransientEBADFOrClosedPipe(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transient error
	}{
		{"fork/exec EBADF from the CI flake", forkExecGitError(syscall.EBADF)},
		{"pipe read EBADF", &os.PathError{Op: "read", Path: "|0", Err: syscall.EBADF}},
		{"bare EBADF errno", syscall.EBADF},
		{"closed pipe", io.ErrClosedPipe},
		{"wrapped closed pipe", fmt.Errorf("copy git output: %w", io.ErrClosedPipe)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedRunner{attempts: []attempt{{nil, tc.transient}, {[]byte("recovered"), nil}}}
			out, err := gittest.CaptureWithEBADFRetry(runner.run)
			if err != nil || string(out) != "recovered" {
				t.Errorf("CaptureWithEBADFRetry after one %s = (%q, %v), want the retry's (\"recovered\", nil)", tc.name, out, err)
			}
			if runner.calls != 2 {
				t.Errorf("the runner ran %d times, want 2: the failed attempt and one retry", runner.calls)
			}
		})
	}
}

func TestC1834_002_CaptureWithEBADFRetryReturnsAPersistentEBADFAfterExactlyOneRetry(t *testing.T) {
	for _, tc := range []struct {
		name       string
		persistent error
		target     error
		wantText   string
	}{
		{"fork/exec EBADF", forkExecGitError(syscall.EBADF), syscall.EBADF, "fork/exec /usr/bin/git: bad file descriptor"},
		{"closed pipe", io.ErrClosedPipe, io.ErrClosedPipe, "closed pipe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedRunner{attempts: []attempt{{nil, tc.persistent}, {nil, tc.persistent}}}
			_, err := gittest.CaptureWithEBADFRetry(runner.run)
			if runner.calls != 2 {
				t.Errorf("the runner ran %d times under a persistent %s, want exactly 2", runner.calls, tc.name)
			}
			if !errors.Is(err, tc.target) {
				t.Fatalf("a persistent %s returned %v, want an error that is %v", tc.name, err, tc.target)
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("the returned error %q does not name %q", err, tc.wantText)
			}
		})
	}
}

func TestC1834_003_CaptureWithEBADFRetryNeverRetriesANonEBADFError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"git exit status", errors.New("exit status 128")},
		{"fork/exec EMFILE", forkExecGitError(syscall.EMFILE)},
		{"broken pipe", forkExecGitError(syscall.EPIPE)},
		{"file already closed", os.ErrClosed},
		{"git not on PATH", &exec.Error{Name: "git", Err: exec.ErrNotFound}},
		{"deadline exceeded", context.DeadlineExceeded},
		{"text that only looks like EBADF", errors.New("fork/exec /usr/bin/git: bad file descriptor")},
		{"clean first attempt", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedRunner{attempts: []attempt{{[]byte("partial"), tc.err}, {[]byte("retried"), nil}}}
			out, err := gittest.CaptureWithEBADFRetry(runner.run)
			if runner.calls != 1 {
				t.Errorf("the runner ran %d times after a %s, want 1: a non-EBADF error is never retried", runner.calls, tc.name)
			}
			if !errors.Is(err, tc.err) || string(out) != "partial" {
				t.Errorf("CaptureWithEBADFRetry = (%q, %v), want the first attempt's (\"partial\", %v) unchanged", out, err, tc.err)
			}
		})
	}

	t.Run("a real git that exits non-zero", func(t *testing.T) {
		calls := 0
		absent := filepath.Join(t.TempDir(), "absent")
		out, err := gittest.CaptureWithEBADFRetry(func() ([]byte, error) {
			calls++
			return exec.Command("git", "-C", absent, "status").CombinedOutput()
		})
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("git -C %s status returned %v, want its *exec.ExitError passed through", absent, err)
		}
		if calls != 1 {
			t.Errorf("git ran %d times after exiting non-zero, want 1", calls)
		}
		if len(out) == 0 {
			t.Errorf("git's own output did not pass through on its failure")
		}
	})
}

func TestC1834_004_IsEBADFLikeClassifiesOnlyEBADFAndClosedPipe(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"bare EBADF errno", syscall.EBADF, true},
		{"fork/exec EBADF", forkExecGitError(syscall.EBADF), true},
		{"wrapped EBADF", fmt.Errorf("git init: %w", forkExecGitError(syscall.EBADF)), true},
		{"closed pipe", io.ErrClosedPipe, true},
		{"wrapped closed pipe", fmt.Errorf("copy: %w", io.ErrClosedPipe), true},
		{"nil", nil, false},
		{"broken pipe", syscall.EPIPE, false},
		{"too many open files", forkExecGitError(syscall.EMFILE), false},
		{"no such file", syscall.ENOENT, false},
		{"file already closed", os.ErrClosed, false},
		{"end of file", io.EOF, false},
		{"git not on PATH", exec.ErrNotFound, false},
		{"git exit status", errors.New("exit status 128"), false},
		{"text that only looks like EBADF", errors.New("bad file descriptor"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := gittest.IsEBADFLike(tc.err); got != tc.want {
				t.Errorf("IsEBADFLike(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestC1834_005_FixtureAndRepoGitRetryAForcedFirstExecEBADFOnce(t *testing.T) {
	frozen := []string{
		"TestFixture_RetriesAGitInitWhoseFirstExecFailsTransiently",
		"TestFixture_PersistentEBADFFailsTheTestNamingTheError",
		"TestRepoGit_NeverRetriesANonEBADFFailure",
	}
	out, err := runGo(t, "test", "-count=1", "-v", "-run", "^("+strings.Join(frozen, "|")+")$", "./internal/gittest")
	if err != nil {
		t.Fatalf("the gittest exec-fault tests fail: %v\n%s", err, out)
	}
	requireEveryTestPassed(t, out, frozen)
}

type shipRetryScan struct {
	ownClassifications   []string
	nonDelegatingHelpers []string
	sharedRuleCalls      int
}

var shipHelperDelegates = map[string]string{
	"captureWithEBADFRetry": "CaptureWithEBADFRetry",
	"isEBADFLike":           "IsEBADFLike",
}

func importedAs(f *ast.File, path string) string {
	for _, spec := range f.Imports {
		if strings.Trim(spec.Path.Value, `"`) != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return filepath.Base(path)
	}
	return ""
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && pkg != "" && id.Name == pkg
}

func isTransientExecTarget(expr ast.Expr) bool {
	return isSelector(expr, "syscall", "EBADF") || isSelector(expr, "io", "ErrClosedPipe")
}

func classifiesTransientExecError(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.CallExpr:
		return isSelector(node.Fun, "errors", "Is") && len(node.Args) == 2 && isTransientExecTarget(node.Args[1])
	case *ast.BinaryExpr:
		return (node.Op == token.EQL || node.Op == token.NEQ) && (isTransientExecTarget(node.X) || isTransientExecTarget(node.Y))
	}
	return false
}

func delegatesTo(fn *ast.FuncDecl, pkg, name string) bool {
	if len(fn.Body.List) != 1 {
		return false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	return ok && isSelector(call.Fun, pkg, name)
}

func scanShipRetryRule(t *testing.T, dir string) shipRetryScan {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go files under %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var scan shipRetryScan
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		gittestName := importedAs(f, gittestImportPath)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if shared, isHelper := shipHelperDelegates[fn.Name.Name]; isHelper && !delegatesTo(fn, gittestName, shared) {
				scan.nonDelegatingHelpers = append(scan.nonDelegatingHelpers, fn.Name.Name+" at "+fset.Position(fn.Pos()).String())
			}
			isTestBody := strings.HasPrefix(fn.Name.Name, "Test")
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && isSelector(call.Fun, gittestName, "CaptureWithEBADFRetry") {
					scan.sharedRuleCalls++
				}
				if !isTestBody && classifiesTransientExecError(n) {
					scan.ownClassifications = append(scan.ownClassifications, fn.Name.Name+" at "+fset.Position(n.Pos()).String())
				}
				return true
			})
		}
	}
	return scan
}

func TestC1834_006_ShipsEBADFRetryIsGittestsOneRuleWithNoSecondCopy(t *testing.T) {
	shipDir := filepath.Join(goModuleRoot(t), "internal", "phases", "ship")
	scan := scanShipRetryRule(t, shipDir)
	if len(scan.ownClassifications) != 0 {
		t.Errorf("ship still classifies EBADF/closed-pipe itself, a second copy of gittest's rule: %v", scan.ownClassifications)
	}
	if len(scan.nonDelegatingHelpers) != 0 {
		t.Errorf("ship declares retry helpers that do not just return gittest's: %v", scan.nonDelegatingHelpers)
	}
	if scan.sharedRuleCalls == 0 {
		t.Errorf("no ship code calls gittest.CaptureWithEBADFRetry, so ship's retry is not gittest's rule")
	}

	shipRetryContract := []string{
		"TestCaptureWithEBADFRetry_RetriesOnceOnEBADF",
		"TestCaptureWithEBADFRetry_RetriesOnceOnClosedPipe",
		"TestCaptureWithEBADFRetry_PersistentEBADF_FailsAfterOneRetry",
		"TestCaptureWithEBADFRetry_NonEBADFError_NoRetry",
		"TestCaptureWithEBADFRetry_SuccessFirstTry_NoRetry",
	}
	out, err := runGo(t, "test", "-count=1", "-tags", "integration", "-v", "-run", "^("+strings.Join(shipRetryContract, "|")+")$", "./internal/phases/ship")
	if err != nil {
		t.Fatalf("ship's captureWithEBADFRetry contract fails on the shared rule: %v\n%s", err, out)
	}
	requireEveryTestPassed(t, out, shipRetryContract)
}

var apicoverSummary = regexp.MustCompile(`summary: (\d+) exported, (\d+) covered, 0 uncovered, 0 false-green`)

func TestC1834_007_GittestApicoverGateNamesAndCoversTheSharedRetryExports(t *testing.T) {
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "coverage.txt")
	if out, err := runGo(t, "test", "-count=1", "-coverprofile="+profile, "./internal/gittest"); err != nil {
		t.Fatalf("go test -coverprofile ./internal/gittest: %v\n%s", err, out)
	}
	funcs, err := runGo(t, "tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover -func: %v\n%s", err, funcs)
	}
	funcReport := filepath.Join(scratch, "coverage.func.txt")
	if err := os.WriteFile(funcReport, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, export := range []string{"CaptureWithEBADFRetry", "IsEBADFLike"} {
		m := regexp.MustCompile(`/internal/gittest/[^/\s]+\.go:\d+:\s+` + export + `\s+(\d+\.\d+)%`).FindStringSubmatch(funcs)
		if m == nil || m[1] == "0.0" {
			t.Errorf("gittest's own tests do not cover %s (cover -func row %q)", export, m)
		}
	}
	gittestDir := filepath.Join(goModuleRoot(t), "internal", "gittest")
	report, err := runGo(t, "run", "./cmd/apicover", "-enforce", "-cover", funcReport, gittestDir)
	if err != nil {
		t.Fatalf("apicover -enforce on gittest refuses: %v\n%s", err, report)
	}
	m := apicoverSummary.FindStringSubmatch(report)
	if m == nil || m[1] != m[2] {
		t.Errorf("apicover on gittest does not report every export named and covered:\n%s", report)
	}
}
