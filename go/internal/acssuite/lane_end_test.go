package acssuite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestRunGoTest_PredicatesReceiveTheLaneEnd(t *testing.T) {
	t.Setenv(ipcenv.ACSLaneEndKey, "2001-01-01T00:00:00Z")
	var envs [][]string
	before := time.Now()

	if _, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoTimeout: time.Hour, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}

	if len(envs) == 0 {
		t.Fatal("no scope ran")
	}
	first := effectiveEnv(envs[0])[ipcenv.ACSLaneEndKey]
	end, err := time.Parse(time.RFC3339Nano, first)
	if err != nil {
		t.Fatalf("%s = %q, want an RFC 3339 time: %v", ipcenv.ACSLaneEndKey, first, err)
	}
	if d := end.Sub(before); d < time.Hour || d > time.Hour+time.Minute {
		t.Errorf("lane end is %s after the start, want the whole 1h budget", d)
	}
	for _, raw := range envs {
		if got := effectiveEnv(raw)[ipcenv.ACSLaneEndKey]; got != first {
			t.Errorf("a scope saw lane end %q, want the one shared deadline %q", got, first)
		}
		if n := strings.Count(strings.Join(raw, "\n"), ipcenv.ACSLaneEndKey+"="); n != 1 {
			t.Errorf("%s appears %d times in the predicate env, want once: the host copy must be dropped", ipcenv.ACSLaneEndKey, n)
		}
	}
}

func TestForwardableKeys_RefuseTheLaneEndKey(t *testing.T) {
	forward, refusals := forwardableKeys([]string{ipcenv.ACSLaneEndKey})
	if len(forward) != 0 || len(refusals) != 1 {
		t.Errorf("forward=%v refusals=%v, want the lane end refused as an operator key", forward, refusals)
	}
}

func blockingFakeGo(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/bin:/usr/bin")
}

func outputEvent(t *testing.T, pkg, test, text string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"Action": "output", "Package": pkg, "Test": test, "Output": text})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func hungPredicateScope(t *testing.T, calls *int) func(context.Context, string, string, []string) (string, error) {
	t.Helper()
	pkg := acsPkgBase + "cycle9"
	return func(_ context.Context, _, pattern string, env []string) (string, error) {
		if pattern != CyclePackage(9) {
			return goStream(goLine(acsPkgBase+"regression/a", "TestA_001_Fine", "run"), goLine(acsPkgBase+"regression/a", "TestA_001_Fine", "pass")), nil
		}
		*calls++
		if effectiveEnv(env)[ipcenv.ACSLaneEndKey] == "" {
			t.Error("the scope got no lane end")
		}
		_, stderr, _, err := acsassert.SubprocessOutput("go", "test", "./x")
		return goStream(
			goLine(pkg, "TestC9_001_Hangs", "run"),
			outputEvent(t, pkg, "TestC9_001_Hangs", "go test failed: "+err.Error()+"\n"+stderr),
			goLine(pkg, "TestC9_001_Hangs", "fail"),
		), os.ErrInvalid
	}
}

func TestRunGoTest_AHungPredicateLeavesTheOtherScopesGreen(t *testing.T) {
	blockingFakeGo(t)
	t.Setenv(ipcenv.ACSLaneEndKey, time.Now().Add(2*time.Second).Format(time.RFC3339Nano))
	calls := 0

	v, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoTimeout: 20 * time.Second, GoExec: hungPredicateScope(t, &calls)})

	if err != nil {
		t.Fatal(err)
	}
	for _, r := range v.Results {
		if strings.HasPrefix(r.ACID, acsverdict.SyntheticRedPrefix+"go-lane-") {
			t.Errorf("harness red %s: one hung predicate must not turn the lane into harness reds", r.ACID)
		}
	}
	if red := resultByACID(t, v, "cycle9/TestC9_001_Hangs"); red.ResultStr != "red" || !strings.Contains(red.EvidenceExcerpt, acsverdict.SubprocessBoundMarker) {
		t.Errorf("hung predicate = {result:%q evidence:%q}, want one named red that carries the bound marker", red.ResultStr, red.EvidenceExcerpt)
	}
	if green := resultByACID(t, v, "a/TestA_001_Fine"); green.ResultStr != "green" {
		t.Errorf("the scope after the hang is %q, want green", green.ResultStr)
	}
	if calls != 1 {
		t.Errorf("the hung scope ran %d times, want once: a bound hit is never retried", calls)
	}
}

func TestIsRetryable_ABoundRedBlocksTheRetryOfItsWholeScope(t *testing.T) {
	bound := Result{ACID: "cycle9/TestHang", ResultStr: "red", fullEvidence: "x " + acsverdict.SubprocessBoundMarker}
	flaky := Result{ACID: "cycle9/TestFlaky", ResultStr: "red", fullEvidence: "plain failure"}
	if !isRetryable([]Result{flaky}) {
		t.Error("an ordinary red must stay retryable")
	}
	if isRetryable([]Result{flaky, bound}) {
		t.Error("a scope with a bound red was retried: the hang would repeat and spend the lane")
	}
}
