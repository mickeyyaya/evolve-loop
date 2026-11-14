package commitgate

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type lintCall struct {
	name, dir string
	args      []string
}

func lintRunner(calls *[]lintCall, code int, stdout, stderr string, err error) Runner {
	return func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, out, errOut io.Writer) (int, error) {
		*calls = append(*calls, lintCall{name, dir, args})
		_, _ = io.WriteString(out, stdout)
		_, _ = io.WriteString(errOut, stderr)
		return code, err
	}
}

func TestLintPackages_ACleanRunReturnsNoFindings(t *testing.T) {
	t.Parallel()
	var calls []lintCall
	out, err := Options{Runner: lintRunner(&calls, 0, "", "", nil), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a", "./b"})
	if out != "" || err != nil {
		t.Fatalf("out=%q err=%v", out, err)
	}
	want := "golangci-lint run --allow-serial-runners ./a ./b"
	if len(calls) != 1 || calls[0].dir != "/mod" || calls[0].name+" "+strings.Join(calls[0].args, " ") != want {
		t.Fatalf("calls=%v, want %q in /mod", calls, want)
	}
}

func TestLintPackages_FindingsComeBackAsTextNotAnError(t *testing.T) {
	t.Parallel()
	var calls []lintCall
	out, err := Options{Runner: lintRunner(&calls, 1, "x.go:3:2: Error return value is not checked\n", "", nil), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if err != nil || !strings.Contains(out, "x.go:3:2: Error return value is not checked") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestLintPackages_AToolFailureIsAnErrorWithItsOutput(t *testing.T) {
	t.Parallel()
	var calls []lintCall
	out, err := Options{Runner: lintRunner(&calls, 3, "", "config broken\n", nil), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if err == nil || !strings.Contains(err.Error(), "exit 3") || !strings.Contains(out, "config broken") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	var none []lintCall
	_, err = Options{Runner: lintRunner(&none, -1, "", "", errors.New("boom")), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if err == nil || !strings.Contains(err.Error(), "boom") || errors.Is(err, core.ErrLintToolMissing) {
		t.Fatalf("err=%v", err)
	}
}

func TestLintPackages_AMissingToolIsTold(t *testing.T) {
	t.Parallel()
	var calls []lintCall
	_, err := Options{Runner: lintRunner(&calls, -1, "", "", &exec.Error{Name: "golangci-lint", Err: exec.ErrNotFound}), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if !errors.Is(err, core.ErrLintToolMissing) {
		t.Fatalf("err=%v, want core.ErrLintToolMissing", err)
	}
}

func TestLintPackages_APastBudgetRunIsALockWait(t *testing.T) {
	t.Parallel()
	run := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		<-ctx.Done()
		return -1, ctx.Err()
	}
	_, err := Options{Runner: run, LintBudget: 20 * time.Millisecond}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if !errors.Is(err, core.ErrLintLockWait) || !strings.Contains(err.Error(), "20ms") {
		t.Fatalf("err=%v, want core.ErrLintLockWait naming 20ms", err)
	}
}

func TestLintPackages_TheExportedRunnerUsesTheHostTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := LintPackages(context.Background(), t.TempDir(), []string{"./."})
	if !errors.Is(err, core.ErrLintToolMissing) {
		t.Fatalf("err=%v, want core.ErrLintToolMissing when golangci-lint is not on PATH", err)
	}
}

func TestLintPackages_AnExitOneWithNoTextStillCountsAsAFinding(t *testing.T) {
	t.Parallel()
	var calls []lintCall
	out, err := Options{Runner: lintRunner(&calls, 1, "", "", nil), LintBudget: time.Minute}.lintPackages(context.Background(), "/mod", []string{"./a"})
	if err != nil || out == "" {
		t.Fatalf("out=%q err=%v, want a finding text", out, err)
	}
}

func TestRunGolangciLint_AToolFailureIsLoggedWithItsExit(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir())
	var calls []lintCall
	o.Runner = lintRunner(&calls, 3, "", "config broken\n", nil)
	res := &Result{}

	ok := o.runGolangciLint(context.Background(), o.RepoRoot, []string{"./."}, res)

	joined := strings.Join(res.Logs, "\n")
	if ok || !strings.Contains(joined, "golangci-lint failed") || !strings.Contains(joined, "config broken") || !strings.Contains(joined, "exit 3") {
		t.Fatalf("ok=%v logs=%v", ok, res.Logs)
	}
}
