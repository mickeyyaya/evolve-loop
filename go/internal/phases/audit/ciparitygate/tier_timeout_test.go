package ciparitygate

import (
	"context"
	"io"
	"slices"
	"testing"
	"time"
)

func timeoutValue(t *testing.T, args []string) string {
	t.Helper()
	at := slices.Index(args, "-timeout")
	if at < 0 || at+1 >= len(args) {
		t.Fatalf("argv %v has no -timeout flag, so go test falls back to its 10 minute default", args)
	}
	return args[at+1]
}

func tierArgvs(t *testing.T, budget time.Duration, script []step) [][]string {
	t.Helper()
	root, _ := goWorktree(t)
	timeouts := DefaultTimeouts()
	timeouts.TierAttempt = budget
	var argvs [][]string
	idx := 0
	run := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		argvs = append(argvs, slices.Clone(args))
		s := script[idx]
		idx++
		_, _ = io.WriteString(so, s.Out)
		return s.Code, nil
	}
	g, _ := observed(t, run, fixedSet("./internal/widget/..."), WithTimeouts(timeouts))
	_, _ = g.IntegrationTier(tierRequest(root, t.TempDir()))
	return argvs
}

func TestIntegrationTier_FirstAttemptPassesGoTestATimeoutUnderItsBudget(t *testing.T) {
	t.Parallel()
	argvs := tierArgvs(t, 15*time.Minute, []step{{0, "ok\n"}})
	if len(argvs) != 1 {
		t.Fatalf("calls = %d, want 1", len(argvs))
	}
	if got := timeoutValue(t, argvs[0]); got != "14m0s" {
		t.Errorf("-timeout = %s, want the 15m budget less the 1m margin", got)
	}
}

func TestIntegrationTier_RetakePassesTheSameTimeout(t *testing.T) {
	t.Parallel()
	argvs := tierArgvs(t, 30*time.Minute, []step{{1, "--- FAIL: TestFlaky (0.00s)\n"}, {0, "ok\n"}})
	if len(argvs) != 2 {
		t.Fatalf("calls = %d, want 2", len(argvs))
	}
	for i, argv := range argvs {
		if got := timeoutValue(t, argv); got != "29m0s" {
			t.Errorf("attempt %d -timeout = %s, want the 30m budget less the 1m margin", i+1, got)
		}
	}
}

func TestTierTimeout_AFloorKeepsABudgetUnderTheMarginPositive(t *testing.T) {
	t.Parallel()
	cases := map[time.Duration]time.Duration{
		15 * time.Minute: 14 * time.Minute,
		time.Minute:      time.Minute,
		time.Nanosecond:  time.Nanosecond,
	}
	for budget, want := range cases {
		if got := tierTimeout(budget); got != want {
			t.Errorf("tierTimeout(%s) = %s, want %s; zero would disable the go test timeout", budget, got, want)
		}
	}
}
