package acssuite

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

const (
	cycleScope      = "cycle"
	regressionScope = "regression"
	redteamScope    = "redteam"
	generousBudget  = 5 * time.Second
	overlapGrace    = 300 * time.Millisecond
	holdWhileInView = 100 * time.Millisecond
)

var passOf = map[string]string{
	cycleScope:      goStream(goLine(acsPkgBase+"cycle2579", "TestC2579_001_Own", "pass")),
	regressionScope: goStream(goLine(acsPkgBase+"regression/cycle42", "TestC42_001_Durable", "pass")),
	redteamScope:    goStream(goLine(acsPkgBase+"redteam", "TestRedTeam_001_Standing", "pass")),
}

func scopeOf(pattern string) string {
	switch {
	case strings.HasPrefix(pattern, "./acs/regression/"):
		return regressionScope
	case strings.HasPrefix(pattern, "./acs/redteam"):
		return redteamScope
	default:
		return cycleScope
	}
}

type inflight struct {
	mu      sync.Mutex
	current int
	peak    int
	order   []string
	pair    chan struct{}
	pairing sync.Once
}

func newInflight() *inflight { return &inflight{pair: make(chan struct{})} }

func (f *inflight) enter(pattern string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current++
	f.peak = max(f.peak, f.current)
	f.order = append(f.order, scopeOf(pattern))
	if f.current >= 2 {
		f.pairing.Do(func() { close(f.pair) })
	}
}

func (f *inflight) leave() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current--
}

func (f *inflight) peakSeen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peak
}

func (f *inflight) startOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

type scopeExec func(ctx context.Context, pattern string) (string, error)

func runSuite(t *testing.T, exec scopeExec) Verdict {
	t.Helper()
	v, err := Run(Options{
		Root:      t.TempDir(),
		Cycle:     2579,
		GoTimeout: generousBudget,
		GoExec: func(ctx context.Context, _ string, pattern string, _ []string) (string, error) {
			return exec(ctx, pattern)
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return v
}

func setLaneShare(t *testing.T, width string) {
	t.Helper()
	t.Setenv(ipcenv.FleetWidthKey, width)
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
}

func requireParallelHost(t *testing.T) {
	t.Helper()
	if runtime.NumCPU() < 2 {
		t.Skip("a host with one CPU has no second lane CPU for a second scope")
	}
}

func resultOrder(v Verdict) []string {
	order := make([]string, len(v.Results))
	for i, r := range v.Results {
		order[i] = r.ACID
	}
	return order
}

func TestC2579_001_IndependentPredicateScopesRunAtTheSameTime(t *testing.T) {
	requireParallelHost(t)
	setLaneShare(t, "1")
	probe := newInflight()

	v := runSuite(t, func(ctx context.Context, pattern string) (string, error) {
		probe.enter(pattern)
		defer probe.leave()
		select {
		case <-probe.pair:
		case <-ctx.Done():
		}
		return passOf[scopeOf(pattern)], nil
	})

	if probe.peakSeen() < 2 {
		t.Errorf("the ACS suite ran its %d predicate scopes one after another (peak in flight %d)", len(passOf), probe.peakSeen())
	}
	if v.Verdict != "PASS" || v.GreenCount != len(passOf) {
		t.Errorf("verdict=%q green=%d red=%d, want PASS with %d green: %v", v.Verdict, v.GreenCount, v.RedCount, len(passOf), resultOrder(v))
	}
}

func TestC2579_002_VerdictOrderFollowsTheScopeListWhateverScopeFinishesFirst(t *testing.T) {
	requireParallelHost(t)
	setLaneShare(t, "1")
	redteamDone := make(chan struct{})

	v := runSuite(t, func(ctx context.Context, pattern string) (string, error) {
		if scopeOf(pattern) == redteamScope {
			defer close(redteamDone)
			return passOf[redteamScope], nil
		}
		if scopeOf(pattern) == cycleScope {
			select {
			case <-redteamDone:
			case <-ctx.Done():
			}
		}
		return passOf[scopeOf(pattern)], nil
	})

	want := []string{"cycle2579/TestC2579_001_Own", "cycle42/TestC42_001_Durable", "redteam/TestRedTeam_001_Standing"}
	if got := strings.Join(resultOrder(v), ","); got != strings.Join(want, ",") {
		t.Errorf("result order = %s, want the scope-list order %s", got, strings.Join(want, ","))
	}
}

func TestC2579_003_AFailingScopeStaysRedWhileTheOtherScopesStillRun(t *testing.T) {
	setLaneShare(t, "1")
	var mu sync.Mutex
	ran := map[string]bool{}

	v := runSuite(t, func(_ context.Context, pattern string) (string, error) {
		mu.Lock()
		ran[scopeOf(pattern)] = true
		mu.Unlock()
		if scopeOf(pattern) == regressionScope {
			return goStream(goLine(acsPkgBase+"regression/cycle42", "TestC42_001_Durable", "output"), goLine(acsPkgBase+"regression/cycle42", "TestC42_001_Durable", "fail")), &fakeExitErr{1}
		}
		return passOf[scopeOf(pattern)], nil
	})

	if v.Verdict != "FAIL" || v.ShipEligible || v.RedCount != 1 {
		t.Errorf("verdict=%q ship=%v red=%d, want FAIL, not ship-eligible, exactly 1 red: %v", v.Verdict, v.ShipEligible, v.RedCount, resultOrder(v))
	}
	mu.Lock()
	defer mu.Unlock()
	for _, scope := range []string{cycleScope, regressionScope, redteamScope} {
		if !ran[scope] {
			t.Errorf("scope %s never ran after another scope failed", scope)
		}
	}
}

func TestC2579_004_ABudgetOfOneCPUKeepsEveryScopeSerial(t *testing.T) {
	setLaneShare(t, "1000000")
	probe := newInflight()

	v := runSuite(t, func(ctx context.Context, pattern string) (string, error) {
		probe.enter(pattern)
		defer probe.leave()
		grace, cancel := context.WithTimeout(ctx, overlapGrace)
		defer cancel()
		select {
		case <-probe.pair:
		case <-grace.Done():
		}
		return passOf[scopeOf(pattern)], nil
	})

	if probe.peakSeen() != 1 {
		t.Errorf("a lane that holds one CPU ran %d scopes at once; the lane budget must bound how many scopes run together", probe.peakSeen())
	}
	if v.Verdict != "PASS" {
		t.Errorf("verdict=%q, want PASS: %v", v.Verdict, resultOrder(v))
	}
}

func TestC2579_005_AScopeThatNeverFinishesSpendsTheSharedBudgetAndIsNamedRed(t *testing.T) {
	requireParallelHost(t)
	setLaneShare(t, "1")

	v, err := Run(Options{
		Root:      t.TempDir(),
		Cycle:     2579,
		GoTimeout: 500 * time.Millisecond,
		GoExec: func(ctx context.Context, _ string, pattern string, _ []string) (string, error) {
			if scopeOf(pattern) == regressionScope {
				<-ctx.Done()
				return "", ctx.Err()
			}
			return passOf[scopeOf(pattern)], nil
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if v.Verdict != "FAIL" || v.ShipEligible {
		t.Fatalf("verdict=%q ship=%v, want FAIL: a scope that never finished must not pass", v.Verdict, v.ShipEligible)
	}
	named := false
	for _, r := range v.Results {
		named = named || (r.ResultStr == "red" && strings.Contains(r.ACID, "go-lane-scope-failed/acs/regression"))
	}
	if !named {
		t.Errorf("no red names the stuck scope: %v", resultOrder(v))
	}
}

func TestScopes_StartInScopeListOrderUnderAOneCPUShare(t *testing.T) {
	setLaneShare(t, "1000000")
	probe := newInflight()

	runSuite(t, func(_ context.Context, pattern string) (string, error) {
		probe.enter(pattern)
		defer probe.leave()
		return passOf[scopeOf(pattern)], nil
	})

	want := []string{cycleScope, regressionScope, redteamScope}
	if got := probe.startOrder(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("scopes started in order %v, want the scope-list order %v", got, want)
	}
}

func TestScopes_AShareOfTwoCPUsRunsAtMostTwoScopesAtOnce(t *testing.T) {
	if runtime.NumCPU() < 4 {
		t.Skip("a share of two CPUs needs a host of at least four")
	}
	width := runtime.NumCPU() / 2
	if lanebudget.Procs(runtime.NumCPU(), width) != 2 {
		t.Skipf("no fleet width gives this host a two-CPU share")
	}
	setLaneShare(t, strconv.Itoa(width))
	probe := newInflight()

	v := runSuite(t, func(ctx context.Context, pattern string) (string, error) {
		probe.enter(pattern)
		defer probe.leave()
		select {
		case <-probe.pair:
		case <-ctx.Done():
		}
		select {
		case <-time.After(holdWhileInView):
		case <-ctx.Done():
		}
		return passOf[scopeOf(pattern)], nil
	})

	if probe.peakSeen() != 2 {
		t.Errorf("peak scopes in flight = %d, want exactly 2 for 3 scopes on a two-CPU share", probe.peakSeen())
	}
	if v.Verdict != "PASS" {
		t.Errorf("verdict=%q, want PASS: %v", v.Verdict, resultOrder(v))
	}
}
