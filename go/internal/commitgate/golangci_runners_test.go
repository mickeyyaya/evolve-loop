package commitgate

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_GolangciLintWaitsForAnotherRunInsteadOfFailing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go", "golangci-lint")
	o.Reviewers = "code-simplifier,go-reviewer"
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
	}}
	o.Runner = sr.run()

	o.Run(context.Background())

	var lint string
	for _, c := range sr.calls {
		if strings.HasPrefix(c, "golangci-lint ") {
			lint = c
		}
	}
	if !strings.HasPrefix(lint, "golangci-lint run --allow-serial-runners ") {
		t.Fatalf("golangci-lint ran as %q; without --allow-serial-runners it exits with an error whenever another golangci-lint on the host holds its lock", lint)
	}
}

func goLintOpts(t *testing.T, run func(ctx context.Context, name string) (int, error)) Options {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	o := baseOpts(root, "shasum", "go", "golangci-lint")
	o.Reviewers = "code-simplifier,go-reviewer"
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: "diff\n", exit: 1},
		{matchPrefix: "gofmt -s -l", stdout: ""},
	}}
	scripted := sr.run()
	o.Runner = func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "golangci-lint" {
			return run(ctx, name)
		}
		return scripted(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
	return o
}

func TestRun_GolangciLintRunsUnderABoundedContext(t *testing.T) {
	t.Parallel()
	var bounded bool
	o := goLintOpts(t, func(ctx context.Context, _ string) (int, error) {
		_, bounded = ctx.Deadline()
		return 0, nil
	})

	o.Run(context.Background())

	if !bounded {
		t.Fatal("golangci-lint ran with no deadline; behind another run's lock --allow-serial-runners waits forever")
	}
}

func TestRun_AGolangciLintThatOutwaitsItsBudgetFailsTheLaneLoudly(t *testing.T) {
	t.Parallel()
	o := goLintOpts(t, func(ctx context.Context, _ string) (int, error) {
		<-ctx.Done()
		return -1, nil
	})
	o.LintBudget = 50 * time.Millisecond

	res := o.Run(context.Background())

	if res.ExitCode != ExitFail {
		t.Fatalf("ExitCode = %d, want %d: a lint that never got the lock must fail the lane", res.ExitCode, ExitFail)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "golangci-lint waited 50ms for another run's lock") {
		t.Fatalf("logs do not say the lint ran out its budget:\n%s", logs)
	}
}

func TestRun_TheLintBudgetDefaultsWhenUnset(t *testing.T) {
	t.Parallel()
	var left time.Duration
	o := goLintOpts(t, func(ctx context.Context, _ string) (int, error) {
		if d, ok := ctx.Deadline(); ok {
			left = time.Until(d)
		}
		return 0, nil
	})

	o.Run(context.Background())

	if left <= defaultLintBudget-time.Minute || left > defaultLintBudget {
		t.Fatalf("lint deadline %s away, want about the %s default", left, defaultLintBudget)
	}
}
