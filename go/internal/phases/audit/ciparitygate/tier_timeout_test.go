package ciparitygate

import (
	"context"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
)

func timeoutValue(t *testing.T, args []string) string {
	t.Helper()
	at := slices.Index(args, "-timeout")
	if at < 0 || at+1 >= len(args) {
		t.Fatalf("argv %v has no -timeout flag, so go test falls back to its 10 minute default", args)
	}
	return args[at+1]
}

func tierArgvs(t *testing.T, script []step) [][]string {
	t.Helper()
	root, _ := goWorktree(t)
	var argvs [][]string
	idx := 0
	run := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		argvs = append(argvs, slices.Clone(args))
		s := script[idx]
		idx++
		_, _ = io.WriteString(so, s.Out)
		return s.Code, nil
	}
	g, _ := observed(t, run, fixedSet("./internal/widget/..."))
	_, _ = g.IntegrationTier(tierRequest(root, t.TempDir()))
	return argvs
}

func TestIntegrationTier_FirstAttemptPassesTheSharedPackageTimeout(t *testing.T) {
	t.Parallel()
	argvs := tierArgvs(t, []step{{0, "ok\n"}})
	if len(argvs) != 1 {
		t.Fatalf("calls = %d, want 1", len(argvs))
	}
	if got := timeoutValue(t, argvs[0]); got != addedtests.PackageTimeout {
		t.Errorf("-timeout = %s, want addedtests.PackageTimeout %s", got, addedtests.PackageTimeout)
	}
}

func TestIntegrationTier_RetakePassesTheSameSharedPackageTimeout(t *testing.T) {
	t.Parallel()
	argvs := tierArgvs(t, []step{{1, "--- FAIL: TestFlaky (0.00s)\n"}, {0, "ok\n"}})
	if len(argvs) != 2 {
		t.Fatalf("calls = %d, want 2", len(argvs))
	}
	for i, argv := range argvs {
		if got := timeoutValue(t, argv); got != addedtests.PackageTimeout {
			t.Errorf("attempt %d -timeout = %s, want addedtests.PackageTimeout %s", i+1, got, addedtests.PackageTimeout)
		}
	}
}

func TestDefaultTimeouts_TierAttemptOutlivesThePackageTimeout(t *testing.T) {
	t.Parallel()
	pkg, err := time.ParseDuration(addedtests.PackageTimeout)
	if err != nil {
		t.Fatal(err)
	}
	budget := DefaultTimeouts().TierAttempt
	if budget <= pkg {
		t.Fatalf("TierAttempt %s must exceed the go test timeout %s, or the context kills the run before go test reports a verdict", budget, pkg)
	}
	if budget != pkg+tierAttemptMargin {
		t.Errorf("TierAttempt = %s, want the package timeout %s plus the %s margin", budget, pkg, tierAttemptMargin)
	}
}

func TestBudgetOver_PanicsOnAPackageTimeoutThatIsNotADuration(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("a package timeout that does not parse must stop the build of the default budget, not yield a zero budget")
		}
	}()
	budgetOver("twenty")
}
