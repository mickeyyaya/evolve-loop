package ciparitygate

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

const redRetake = "--- FAIL: TestGenuine (0.01s)\n--- FAIL: TestSecond/sub (0.01s)\nFAIL\texample.com/m/pkg\t2.0s\n"

var (
	genuineKey = baseattr.Key{Package: "example.com/m/pkg", Test: "TestGenuine"}
	secondKey  = baseattr.Key{Package: "example.com/m/pkg", Test: "TestSecond"}
)

type attributionSpy struct {
	calls    int
	reds     baseattr.Reds
	lockHeld bool
	verdict  baseattr.Verdict
	err      error
	lockPath string
}

func (s *attributionSpy) attribute(_ context.Context, r baseattr.Reds) (baseattr.Verdict, error) {
	s.calls++
	s.reds = r
	rel, held, err := flock.TryLock(s.lockPath)
	s.lockHeld = err == nil && held
	if rel != nil {
		rel()
	}
	return s.verdict, s.err
}

func redRedTier(t *testing.T, spy *attributionSpy, retake step) ([]string, error, []string) {
	t.Helper()
	root, _ := goWorktree(t)
	req := tierRequest(root, t.TempDir())
	spy.lockPath = filepath.Join(root, ".evolve", "locks", "integration-tier.lock")
	fn, _, _ := seqRunFunc(t, []step{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, retake})
	g, events := observed(t, fn, fixedSet("./internal/widget/..."), WithAttribution(spy.attribute))
	off, err := g.IntegrationTier(req)
	lines := make([]string, 0, len(*events))
	for _, e := range *events {
		lines = append(lines, string(e.Code)+" keys="+e.Fields["keys"]+" base="+e.Fields["base"]+" main_red="+e.Fields["main_red"]+" took="+e.Fields["took"]+" log="+filepath.Base(e.Fields["log"])+" cause="+e.Fields["cause"]+" err="+e.Fields["err"]+" gate="+e.Fields["gate"]+" reason="+e.Reason)
	}
	return off, err, lines
}

func TestIntegrationTier_AllBaseRedOffendersWarnAndDoNotFail(t *testing.T) {
	t.Parallel()
	spy := &attributionSpy{verdict: baseattr.Verdict{
		BaseRed: []baseattr.Key{genuineKey, secondKey}, MainRed: []baseattr.Key{secondKey}, Base: "b45e", Main: "9a2b", Took: 3 * time.Second,
	}}

	off, err, events := redRedTier(t, spy, step{1, redRetake})

	var notice BaseRed
	if off != nil || !errors.As(err, &notice) {
		t.Fatalf("(%v, %v), want no offender and a BaseRed notice", off, err)
	}
	const names = "example.com/m/pkg.TestGenuine, example.com/m/pkg.TestSecond"
	wantNotice := "each red test of the integration tier is red on the lane base b45e too, so the audit does not blame the lane: " + names
	if err.Error() != wantNotice {
		t.Errorf("notice = %q, want %q", notice, wantNotice)
	}
	want := string(CodeTierBaseRedAttributed) + " keys=" + names + " base=b45e main_red=example.com/m/pkg.TestSecond took=3s log=integration-tier.log cause= err= gate=integration_tier reason=" + wantNotice
	if len(events) != 1 || events[0] != want {
		t.Errorf("events = %q, want only %q", events, want)
	}
	if spy.calls != 1 || !slices.Equal(spy.reds.Keys, []baseattr.Key{genuineKey, secondKey}) || spy.reds.Unnamed ||
		!slices.Equal(spy.reds.Flags, []string{"-race", "-tags", "integration"}) {
		t.Errorf("attribution got %d call(s) with %+v, want one call with the two red keys and the flags of the tier", spy.calls, spy.reds)
	}
	if !spy.lockHeld {
		t.Error("the base run started without the tier lock")
	}
	rel, held, lerr := flock.TryLock(spy.lockPath)
	if lerr != nil || held {
		t.Fatalf("the tier lock is held after the gate returned (held=%v err=%v)", held, lerr)
	}
	rel()
}

func TestIntegrationTier_ARedThatIsNotAllBaseRedFailsAsBefore(t *testing.T) {
	t.Parallel()
	const names = "example.com/m/pkg.TestGenuine, example.com/m/pkg.TestSecond"
	cases := []struct {
		name      string
		verdict   baseattr.Verdict
		err       error
		wantEvent string
	}{
		{name: "each red is lane-only", verdict: baseattr.Verdict{LaneOnly: []baseattr.Key{genuineKey, secondKey}, Base: "b45e"}},
		{name: "one red is lane-only", verdict: baseattr.Verdict{LaneOnly: []baseattr.Key{genuineKey}, BaseRed: []baseattr.Key{secondKey}, Base: "b45e"}},
		{name: "the verdict is unjudged", verdict: baseattr.Verdict{BaseRed: []baseattr.Key{genuineKey, secondKey}, Unjudged: true}},
		{name: "the base run fails", verdict: baseattr.Verdict{BaseRed: []baseattr.Key{genuineKey, secondKey}}, err: errors.New("baseattr: add worktree: exit 128"),
			wantEvent: string(CodeTierBaseRedUnjudged) + " keys=" + names + " base= main_red= took= log=integration-tier.log cause= err=baseattr: add worktree: exit 128 gate=integration_tier" +
				" reason=the red tests of the integration tier could not be run on the lane base, so the audit blames the lane as before: baseattr: add worktree: exit 128"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spy := &attributionSpy{verdict: tc.verdict, err: tc.err}

			off, err, events := redRedTier(t, spy, step{1, redRetake})

			if err != nil || len(off) == 0 || !strings.Contains(strings.Join(off, "\n"), "TestGenuine") {
				t.Fatalf("(%v, %v), want the offenders of the retake", off, err)
			}
			if spy.calls != 1 {
				t.Errorf("attribution calls = %d, want 1", spy.calls)
			}
			wantCodes := []string{string(CodeGateFailed)}
			if tc.wantEvent != "" {
				wantCodes = []string{tc.wantEvent, string(CodeGateFailed)}
			}
			if len(events) != len(wantCodes) || !strings.HasPrefix(events[len(events)-1], string(CodeGateFailed)+" ") || (tc.wantEvent != "" && events[0] != tc.wantEvent) {
				t.Errorf("events = %q, want %q", events, wantCodes)
			}
		})
	}
}

func TestIntegrationTier_ARedWithNoNamedVerdictGetsNoAttribution(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		retake step
	}{
		{name: "a failed test with no package line", retake: step{1, "--- FAIL: TestGenuine (0.01s)\n"}},
		{name: "a panic", retake: step{1, "panic: boom\n" + redRetake}},
		{name: "a failed package with no test", retake: step{1, "FAIL\texample.com/m/pkg\t2.0s\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spy := &attributionSpy{verdict: baseattr.Verdict{BaseRed: []baseattr.Key{genuineKey}}}

			off, err, events := redRedTier(t, spy, tc.retake)

			if err != nil || len(off) == 0 || spy.calls != 0 {
				t.Fatalf("(%v, %v) with %d attribution call(s), want offenders and no base run", off, err, spy.calls)
			}
			if len(events) != 1 || !strings.Contains(events[0], "cause=retake_red ") {
				t.Errorf("events = %q, want one GATE_FAILED with cause retake_red", events)
			}
		})
	}
}

func TestIntegrationTier_ADeadlineRedGetsNoAttribution(t *testing.T) {
	t.Parallel()
	root, _ := goWorktree(t)
	req := tierRequest(root, t.TempDir())
	budget := DefaultTimeouts()
	budget.TierAttempt = time.Nanosecond
	spy := &attributionSpy{verdict: baseattr.Verdict{BaseRed: []baseattr.Key{genuineKey, secondKey}}}
	fn, _, _ := seqRunFunc(t, []step{{1, "--- FAIL: TestSlowRed (0.00s)\n"}, {-1, redRetake + "signal: killed\n"}})
	g, events := observed(t, killedAtDeadline(fn), fixedSet("./internal/widget/..."), WithTimeouts(budget), WithAttribution(spy.attribute))

	off, err := g.IntegrationTier(req)

	if err != nil || len(off) == 0 || spy.calls != 0 {
		t.Fatalf("(%v, %v) with %d attribution call(s), want offenders and no base run", off, err, spy.calls)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["cause"] != "deadline_with_markers" {
		t.Errorf("GATE_FAILED: %+v", e)
	}
}

func TestTierTestFlags_AreTheFlagsOfTheTierRun(t *testing.T) {
	t.Parallel()
	args := tierArgs(nil)
	for _, flag := range tierTestFlags {
		if !slices.Contains(args, flag) {
			t.Errorf("the base run flag %s is not in the tier run %v", flag, args)
		}
	}
	if i := slices.Index(args, "-tags"); i < 0 || args[i+1] != tierTestFlags[slices.Index(tierTestFlags, "-tags")+1] {
		t.Errorf("the base run and the tier run %v use different build tags", args)
	}
}

func TestIntegrationTier_APanicOnTheStderrOfTheRetakeGetsNoAttribution(t *testing.T) {
	t.Parallel()
	root, _ := goWorktree(t)
	spy := &attributionSpy{verdict: baseattr.Verdict{BaseRed: []baseattr.Key{genuineKey, secondKey}}, lockPath: filepath.Join(t.TempDir(), "spy.lock")}
	fn, calls, _ := seqRunFunc(t, []step{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, redRetake}})
	panicOnRetakeStderr := func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		code, err := fn(ctx, name, dir, args, env, stdin, stdout, stderr)
		if *calls == 2 {
			_, _ = io.WriteString(stderr, "panic: boom\n")
		}
		return code, err
	}
	g, _ := observed(t, panicOnRetakeStderr, fixedSet("./internal/widget/..."), WithAttribution(spy.attribute))

	off, err := g.IntegrationTier(tierRequest(root, t.TempDir()))

	if err != nil || len(off) == 0 || spy.calls != 0 {
		t.Fatalf("(%v, %v) with %d attribution call(s), want offenders and no base run for a panic on stderr", off, err, spy.calls)
	}
}

func TestTierEnv_IsTheEnvironmentOfEachTierAttempt(t *testing.T) {
	t.Setenv("CIPARITYGATE_AMBIENT_PROBE", "1")
	root, _ := goWorktree(t)
	fn, _, envs := seqRunFunc(t, []step{{1, "--- FAIL: TestNoisyFirst (0.00s)\n"}, {1, redRetake}})
	g, _ := observed(t, fn, fixedSet("./internal/widget/..."))

	if _, err := g.IntegrationTier(tierRequest(root, t.TempDir())); err != nil {
		t.Fatal(err)
	}

	env := TierEnv()
	if len(env) == 0 || slices.Contains(env, "CIPARITYGATE_AMBIENT_PROBE=1") {
		t.Fatalf("TierEnv() = %v, want the allowlist values with no ambient variable", env)
	}
	if len(*envs) != 2 || !slices.Equal((*envs)[0], env) || !slices.Equal((*envs)[1], env) {
		t.Errorf("the tier attempts ran with %v, want TierEnv() %v for each attempt", *envs, env)
	}
}
