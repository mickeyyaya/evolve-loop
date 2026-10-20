package cliroute_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

var shareChain = []string{"agy", "claude"}

var shareNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func shareBlock(pct int) policy.CLIRouting {
	return policy.CLIRouting{
		CLIs: shareChain, Default: shareChain,
		Agents: map[string]policy.AgentRule{"builder": {CLI: shareChain, Model: "balanced"}},
		Tiers:  map[string]policy.TierRule{"deep": {CLIs: shareChain, SharePct: pct}},
	}
}

func argonWeek(used float64, resetsIn time.Duration) func(string, string, string) (quotastate.UsageWindow, bool, error) {
	return func(_, family, tier string) (quotastate.UsageWindow, bool, error) {
		if family != "agy" || tier != "deep" {
			return quotastate.UsageWindow{}, false, nil
		}
		reset := shareNow.Add(resetsIn)
		return quotastate.UsageWindow{Scope: "Gemini 4 Argon", Kind: quotastate.KindWeek, Family: "agy", Model: "Gemini 4 Argon", PercentUsed: used, ResetsAt: &reset}, true, nil
	}
}

func shareHost(week func(string, string, string) (quotastate.UsageWindow, bool, error)) cliroute.Host {
	return cliroute.Host{TierWeek: week, Now: func() time.Time { return shareNow }}
}

func deepRequest(cycle int) cliroute.Request {
	req := builderRequest(llmroute.Overlay{LadderTier: "deep"})
	req.Cycle = cycle
	return req
}

func TestShare_ADeepWalkOverAHundredDispatchesSendsArgonAboutItsShare(t *testing.T) {
	t.Parallel()
	r := syntheticRouter(t, shareBlock(50), shareHost(argonWeek(0, 7*24*time.Hour)))

	argon := 0
	for cycle := 1; cycle <= 100; cycle++ {
		if d := mustResolve(t, r, deepRequest(cycle)); llmroute.Family(d.Plan.Candidates[0]) == "agy" {
			argon++
		}
	}

	if argon < 40 || argon > 60 {
		t.Fatalf("Argon led %d of 100 deep dispatches, want about the 50%% share (40 to 60)", argon)
	}
}

func TestShare_TheSameDispatchAlwaysGetsTheSameSeat(t *testing.T) {
	t.Parallel()
	r := syntheticRouter(t, shareBlock(50), shareHost(argonWeek(0, 7*24*time.Hour)))

	for cycle := 1; cycle <= 20; cycle++ {
		a, b := mustResolve(t, r, deepRequest(cycle)), mustResolve(t, r, deepRequest(cycle))
		if strings.Join(a.Plan.Candidates, ",") != strings.Join(b.Plan.Candidates, ",") {
			t.Fatalf("cycle %d resolved %v then %v; the share must be deterministic", cycle, a.Plan.Candidates, b.Plan.Candidates)
		}
	}
}

func keptCycle(t *testing.T, r *cliroute.Router) int {
	t.Helper()
	for cycle := 1; cycle <= 100; cycle++ {
		if d := mustResolve(t, r, deepRequest(cycle)); llmroute.Family(d.Plan.Candidates[0]) == "agy" {
			return cycle
		}
	}
	t.Fatal("no cycle in 1..100 keeps Argon at a 50% share")
	return 0
}

func TestShare_APacedGroupGivesTheSeatToTheNextHealthyFamily(t *testing.T) {
	t.Parallel()
	cycle := keptCycle(t, syntheticRouter(t, shareBlock(50), shareHost(argonWeek(0, 7*24*time.Hour))))
	r := syntheticRouter(t, shareBlock(50), shareHost(argonWeek(60, 6*24*time.Hour)))

	d := mustResolve(t, r, deepRequest(cycle))

	requireChain(t, d, "claude", "agy")
	requireTrace(t, d, "share 50%", "pace: agy Gemini 4 Argon used 60% of its week with 14% of the week gone, so it is paced")
}

func TestShare_AShareBucketPastTheShareGivesTheSeatToTheNextFamily(t *testing.T) {
	t.Parallel()
	r := syntheticRouter(t, shareBlock(1), shareHost(argonWeek(0, 7*24*time.Hour)))

	for cycle := 1; cycle <= 10; cycle++ {
		d := mustResolve(t, r, deepRequest(cycle))
		if llmroute.Family(d.Plan.Candidates[0]) == "agy" {
			continue
		}
		requireChain(t, d, "claude", "agy")
		requireTrace(t, d, "share 1%: bucket", "share gives the seat to claude")
		return
	}
	t.Fatal("a 1% share kept Argon on 10 dispatches in a row")
}

func TestShare_WithoutAUsageReadingOnlyTheShareDecides(t *testing.T) {
	t.Parallel()
	for name, week := range map[string]func(string, string, string) (quotastate.UsageWindow, bool, error){
		"no reader": nil,
		"no window": func(string, string, string) (quotastate.UsageWindow, bool, error) {
			return quotastate.UsageWindow{}, false, nil
		},
		"read error": func(string, string, string) (quotastate.UsageWindow, bool, error) {
			return quotastate.UsageWindow{}, false, errors.New("usage file unreadable")
		},
		"no reset": func(string, string, string) (quotastate.UsageWindow, bool, error) {
			return quotastate.UsageWindow{Family: "agy", PercentUsed: 99}, true, nil
		},
	} {
		r := syntheticRouter(t, shareBlock(99), shareHost(week))
		d := mustResolve(t, r, deepRequest(keptCycle(t, r)))
		if llmroute.Family(d.Plan.Candidates[0]) != "agy" || !strings.Contains(strings.Join(d.Trace, "\n"), "the pace does not apply") {
			t.Errorf("%s: chain %v trace %q — with no usable reading the pace must not move the seat, and the trace must say so", name, d.Plan.Candidates, d.Trace)
		}
	}
}

func TestShare_AnotherTierIsUntouched(t *testing.T) {
	t.Parallel()
	r := syntheticRouter(t, shareBlock(1), shareHost(argonWeek(99, time.Hour)))

	d := mustResolve(t, r, builderRequest(llmroute.Overlay{}))

	requireChain(t, d, "agy", "claude")
	if strings.Contains(strings.Join(d.Trace, "\n"), "share") {
		t.Fatalf("a balanced dispatch must not be shared: %q", d.Trace)
	}
}

func TestShare_AChainThatDoesNotLeadWithTheSharedFamilyIsUntouched(t *testing.T) {
	t.Parallel()
	block := shareBlock(1)
	block.Tiers = map[string]policy.TierRule{"deep": {CLIs: []string{"claude", "agy"}, SharePct: 1}}
	r := syntheticRouter(t, block, shareHost(argonWeek(99, time.Hour)))

	for cycle := 1; cycle <= 10; cycle++ {
		d := mustResolve(t, r, deepRequest(cycle))
		requireChain(t, d, "agy", "claude")
		if strings.Contains(strings.Join(d.Trace, "\n"), "share") {
			t.Fatalf("a chain that leads with agy is not the share of claude: %q", d.Trace)
		}
	}
}

func TestCompile_AShareNeedsATierChainAndAPercentInRange(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		rule policy.TierRule
		want string
	}{
		"too high":   {policy.TierRule{CLIs: shareChain, SharePct: 100}, "share_pct 100 is outside 1 to 99"},
		"negative":   {policy.TierRule{CLIs: shareChain, SharePct: -1}, "share_pct -1 is outside 1 to 99"},
		"no chain":   {policy.TierRule{Effort: "high", SharePct: 50}, "share_pct needs clis"},
		"one family": {policy.TierRule{CLIs: []string{"claude"}, SharePct: 50}, "share_pct needs a second family"},
	} {
		block := shareBlock(0)
		block.Tiers = map[string]policy.TierRule{"deep": tc.rule}
		_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t))
		if !hasFinding(findings, cliroute.SeverityError, "cli_routing.tiers.deep.share_pct", tc.want) {
			t.Errorf("%s: findings %+v lack an error naming %q", name, findings, tc.want)
		}
	}
}

func hasFinding(findings []cliroute.Finding, sev cliroute.Severity, key, part string) bool {
	for _, f := range findings {
		if f.Severity == sev && f.Key == key && strings.Contains(f.Message, part) {
			return true
		}
	}
	return false
}

func TestShareNotes_NameEachSharedTierAndItsPace(t *testing.T) {
	t.Parallel()
	r := syntheticRouter(t, shareBlock(50), shareHost(argonWeek(60, 6*24*time.Hour)))

	notes := r.ShareNotes("/project", []string{"agy-tmux", "claude-tmux"})
	if other := r.ShareNotes("/project", []string{"claude-tmux"}); len(other) != 0 {
		t.Fatalf("a chain without agy gets no share note, got %q", other)
	}

	want := "share: when the agent dispatches at deep, agy takes 50% of the seats; pace: agy Gemini 4 Argon used 60% of its week with 14% of the week gone, so it is paced"
	if len(notes) != 1 || notes[0] != want {
		t.Fatalf("notes = %q, want [%q]", notes, want)
	}
}
