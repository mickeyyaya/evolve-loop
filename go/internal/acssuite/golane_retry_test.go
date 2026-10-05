package acssuite

import (
	"context"
	"errors"
	"testing"
)

func cycleScopeRuns(runs []string, errs []error) (func(context.Context, string, string, []string) (string, error), *int) {
	calls := 0
	return func(_ context.Context, _, pattern string, _ []string) (string, error) {
		if pattern != CyclePackage(9) {
			return "", nil
		}
		i := min(calls, len(runs)-1)
		calls++
		return runs[i], errs[i]
	}, &calls
}

func TestRun_AStreamWithAHarnessRedIsNeverRetried(t *testing.T) {
	first := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "fail"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Unfinished", "run"),
	)
	exec, calls := cycleScopeRuns([]string{first, goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "pass"))}, []error{errors.New("exit status 1"), nil})
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: exec})
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Errorf("the scope ran %d times, want once: a stream carrying a harness red is untrustworthy and is never retried", *calls)
	}
	if red := resultByACID(t, v, "cycle9/TestC9_001_Red"); red.ResultStr != "red" || red.Flaky != "" {
		t.Errorf("first-run red = {result:%q flaky:%q}, want it kept red", red.ResultStr, red.Flaky)
	}
}

func TestRun_ARetryThatLeftATestUnfinishedCannotFlipARed(t *testing.T) {
	first := goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "fail"))
	retry := goStream(
		goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "pass"),
		goLine(acsPkgBase+"cycle9", "TestC9_002_Unfinished", "run"),
	)
	exec, calls := cycleScopeRuns([]string{first, retry}, []error{errors.New("exit status 1"), nil})
	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: exec})
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 2 {
		t.Fatalf("the scope ran %d times, want the one bounded retry", *calls)
	}
	red := resultByACID(t, v, "cycle9/TestC9_001_Red")
	if red.ResultStr != "red" || red.Flaky != "" || red.RetryOutcome != "retry-inconclusive" {
		t.Errorf("red after an incomplete retry = {result:%q flaky:%q retry:%q}, want it kept red and the retry inconclusive", red.ResultStr, red.Flaky, red.RetryOutcome)
	}
	if v.ShipEligible {
		t.Error("an incomplete retry made the verdict ship-eligible")
	}
}

func TestRun_TheRetryRunsInTheSameEnvironmentAsTheFirstRun(t *testing.T) {
	root := t.TempDir()
	runs := []string{goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "fail")), goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Red", "fail"))}
	var envs [][]string
	exec := func(_ context.Context, _, pattern string, env []string) (string, error) {
		if pattern != CyclePackage(9) {
			return "", nil
		}
		envs = append(envs, env)
		return runs[len(envs)-1], errors.New("exit status 1")
	}
	if _, err := Run(Options{Root: root, Cycle: 9, GoExec: exec}); err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 {
		t.Fatalf("the scope ran %d times, want the first run and one retry", len(envs))
	}
	first, retry := effectiveEnv(envs[0]), effectiveEnv(envs[1])
	if retry["EVOLVE_PROJECT_ROOT"] != root || len(retry) != len(first) {
		t.Errorf("retry env has %d keys and state root %q, want the first run's %d keys and %q", len(retry), retry["EVOLVE_PROJECT_ROOT"], len(first), root)
	}
}
