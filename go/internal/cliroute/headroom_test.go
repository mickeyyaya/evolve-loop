package cliroute_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const headroomRoot = "/project"

var operatorBuildChain = []string{"agy", "agy-claude", "claude"}

func headroomRule() policy.AgentRule {
	return policy.AgentRule{CLI: operatorBuildChain, Model: "balanced", Order: cliroute.OrderHeadroom}
}

func headroomTable(t *testing.T, rule policy.AgentRule) cliroute.Table {
	t.Helper()
	pol := operatorTable()
	pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": rule}
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	return mustCompile(t, pol, realCatalog(t), profiles.NewFromDir(profileDir))
}

func weekUsed(t *testing.T, used map[string]float64) func(string) (map[string]float64, error) {
	return func(root string) (map[string]float64, error) {
		if root != headroomRoot {
			t.Errorf("the usage read got root %q, want %q", root, headroomRoot)
		}
		return used, nil
	}
}

func laneTimes(lanes ...lanetime.FamilyLane) func(string) ([]lanetime.FamilyLane, error) {
	return func(string) ([]lanetime.FamilyLane, error) { return lanes, nil }
}

func buildDecision(t *testing.T, rule policy.AgentRule, host cliroute.Host) cliroute.Decision {
	t.Helper()
	host.LookPath = everyBinary()
	r := mustRouter(t, headroomTable(t, rule), host)
	return mustResolve(t, r, cliroute.Request{Agent: "builder", Phase: "build", ProjectRoot: headroomRoot, DefaultModel: "balanced"})
}

func requireChain(t *testing.T, d cliroute.Decision, want ...string) {
	t.Helper()
	if got := families(d.Plan.Candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("chain families = %v, want %v (trace %q)", got, want, d.Trace)
	}
}

func requireTrace(t *testing.T, d cliroute.Decision, parts ...string) {
	t.Helper()
	all := strings.Join(d.Trace, "\n")
	for _, p := range parts {
		if !strings.Contains(all, p) {
			t.Fatalf("trace %q does not mention %q", d.Trace, p)
		}
	}
}

func lane(family string, builds, passes int, perPass time.Duration) lanetime.FamilyLane {
	return lanetime.FamilyLane{Family: family, Builds: builds, Passes: passes, Lane: perPass * time.Duration(passes), PerPass: perPass}
}

func TestHeadroom_WithTheWeekAt14PercentAndNoMeasureTheBuildStartsOnClaude(t *testing.T) {
	d := buildDecision(t, headroomRule(), cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": 14})})
	requireChain(t, d, "claude", "agy", "agy-claude")
	requireTrace(t, d, "headroom 60%", "claude week 14%", "no family has a measure")
}

func TestHeadroom_AFamilyAtTheLimitLeavesTheHead(t *testing.T) {
	for _, used := range []float64{60, 99} {
		d := buildDecision(t, headroomRule(), cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": used, "agy": 10})})
		requireChain(t, d, "agy", "agy-claude", "claude")
		requireTrace(t, d, "leaves the head")
	}
	d := buildDecision(t, headroomRule(), cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": 59.9})})
	requireChain(t, d, "claude", "agy", "agy-claude")
}

func TestHeadroom_TheLimitIsThePolicyHeadroomPct(t *testing.T) {
	rule := headroomRule()
	rule.HeadroomPct = 80
	d := buildDecision(t, rule, cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": 70})})
	requireChain(t, d, "claude", "agy", "agy-claude")
	requireTrace(t, d, "headroom 80%")
	d = buildDecision(t, rule, cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": 80})})
	requireChain(t, d, "agy", "agy-claude", "claude")
}

func TestHeadroom_WithAMeasureTheShorterLaneTimeToAPassGoesFirst(t *testing.T) {
	used := map[string]float64{"claude": 14, "agy": 10, "agy-claude": 10}
	d := buildDecision(t, headroomRule(), cliroute.Host{
		WeekUsed:  weekUsed(t, used),
		LaneTimes: laneTimes(lane("agy", 20, 10, 30*time.Minute), lane("claude", 20, 16, 62*time.Minute)),
	})
	requireChain(t, d, "agy", "claude", "agy-claude")
	requireTrace(t, d, "agy 30m0s per PASS (20 builds, 10 passes)", "agy-claude no measure")
	d = buildDecision(t, headroomRule(), cliroute.Host{
		WeekUsed:  weekUsed(t, used),
		LaneTimes: laneTimes(lane("agy-claude", 4, 2, 40*time.Minute), lane("agy", 6, 0, 0)),
	})
	requireChain(t, d, "agy-claude", "claude", "agy")
}

func TestHeadroom_AMeasuredFamilyWithoutHeadroomStillLeavesTheHead(t *testing.T) {
	d := buildDecision(t, headroomRule(), cliroute.Host{
		WeekUsed:  weekUsed(t, map[string]float64{"claude": 14, "agy": 100, "agy-claude": 100}),
		LaneTimes: laneTimes(lane("agy", 20, 10, 30*time.Minute), lane("claude", 20, 16, 62*time.Minute)),
	})
	requireChain(t, d, "claude", "agy", "agy-claude")
}

func TestHeadroom_AFailedUsageReadKeepsThePolicyOrder(t *testing.T) {
	var logged []string
	logf := func(format string, args ...any) { logged = append(logged, format) }
	cases := map[string]cliroute.Host{
		"the read fails":      {WeekUsed: func(string) (map[string]float64, error) { return nil, errors.New("pane timeout") }},
		"no usage reader":     {},
		"no week window":      {WeekUsed: weekUsed(t, map[string]float64{})},
		"no family the chain": {WeekUsed: weekUsed(t, map[string]float64{"codex": 1})},
	}
	for name, host := range cases {
		t.Run(name, func(t *testing.T) {
			logged = nil
			host.Logf = logf
			host.LaneTimes = laneTimes(lane("claude", 20, 16, 10*time.Minute))
			d := buildDecision(t, headroomRule(), host)
			requireChain(t, d, operatorBuildChain...)
			requireTrace(t, d, "the policy order stays")
			if !strings.Contains(strings.Join(logged, "\n"), "the policy order stays") {
				t.Fatalf("the fail-open must be logged, got %q", logged)
			}
		})
	}
}

func TestHeadroom_AFailedLaneTimeReadOrdersWithoutAMeasure(t *testing.T) {
	d := buildDecision(t, headroomRule(), cliroute.Host{
		WeekUsed:  weekUsed(t, map[string]float64{"claude": 14}),
		LaneTimes: func(string) ([]lanetime.FamilyLane, error) { return nil, errors.New("lane-time.json: bad json") },
		Logf:      func(string, ...any) {},
	})
	requireChain(t, d, "claude", "agy", "agy-claude")
	requireTrace(t, d, "lane time: lane-time.json: bad json")
}

func TestHeadroom_TheCliHealthBenchStillDemotesAFamilyInCooldown(t *testing.T) {
	var benchSaw []string
	bench := func(_, _ string, plan llmroute.Plan, _ map[string]string) llmroute.Plan {
		benchSaw = families(plan.Candidates)
		return llmroute.ApplyBench(plan, map[string]time.Time{"claude": time.Now().Add(time.Hour)})
	}
	d := buildDecision(t, headroomRule(), cliroute.Host{WeekUsed: weekUsed(t, map[string]float64{"claude": 14}), Bench: bench})
	if !reflect.DeepEqual(benchSaw, []string{"claude", "agy", "agy-claude"}) {
		t.Fatalf("the bench must see the headroom order, saw %v", benchSaw)
	}
	if got := families(d.Plan.Candidates); got[0] == "claude" {
		t.Fatalf("a benched claude must leave the head after the headroom order, got %v", got)
	}
}

func TestHeadroom_ASeatWithoutTheOrderKeyRoutesAsBefore(t *testing.T) {
	calls := 0
	host := cliroute.Host{
		LookPath: everyBinary(),
		WeekUsed: func(string) (map[string]float64, error) {
			calls++
			return map[string]float64{"claude": 1, "agy": 99, "agy-claude": 99}, nil
		},
		LaneTimes: func(string) ([]lanetime.FamilyLane, error) {
			calls++
			return []lanetime.FamilyLane{lane("claude", 20, 20, time.Minute)}, nil
		},
	}
	f := newOperatorFixture(t)
	plain := policy.AgentRule{CLI: operatorBuildChain, Model: "balanced"}
	withPorts := mustRouter(t, headroomTable(t, plain), host)
	without := mustRouter(t, headroomTable(t, plain), cliroute.Host{LookPath: everyBinary()})
	for _, agent := range f.agents {
		for _, req := range f.requests(agent) {
			got, gotErr := withPorts.Resolve(req)
			want, wantErr := without.Resolve(req)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("%s %+v: with the ports %+v (%v), without %+v (%v)", agent, req, got, gotErr, want, wantErr)
			}
		}
	}
	if calls != 0 {
		t.Fatalf("a table without order keys must never read the usage or the lane time, read %d times", calls)
	}
	requireChain(t, buildDecision(t, plain, host), "agy", "agy-claude", "claude")
}

func TestCompile_TheOrderKeysAreChecked(t *testing.T) {
	cases := []struct {
		name string
		rule policy.AgentRule
		key  string
		says string
	}{
		{"unknown order", policy.AgentRule{CLI: operatorBuildChain, Order: "fastest"}, "cli_routing.agents.builder.order", "fastest"},
		{"limit without the order", policy.AgentRule{CLI: operatorBuildChain, HeadroomPct: 50}, "cli_routing.agents.builder.headroom_pct", "order headroom"},
		{"limit above 100", policy.AgentRule{CLI: operatorBuildChain, Order: cliroute.OrderHeadroom, HeadroomPct: 101}, "cli_routing.agents.builder.headroom_pct", "101"},
		{"negative limit", policy.AgentRule{CLI: operatorBuildChain, Order: cliroute.OrderHeadroom, HeadroomPct: -1}, "cli_routing.agents.builder.headroom_pct", "-1"},
		{"order without a chain", policy.AgentRule{Model: "balanced", Order: cliroute.OrderHeadroom}, "cli_routing.agents.builder.order", "cli"},
	}
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol := operatorTable()
			pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": tc.rule}
			_, findings := cliroute.Compile(pol, realCatalog(t), profiles.NewFromDir(profileDir))
			requireFinding(t, findings, tc.key, cliroute.SeverityError, tc.says)
		})
	}
}

func TestHeadroom_TheDefaultLimitIs60Percent(t *testing.T) {
	if cliroute.DefaultHeadroomPct != 60 {
		t.Fatalf("DefaultHeadroomPct = %d, want 60 (plan D4)", cliroute.DefaultHeadroomPct)
	}
}

func TestHeadroom_ThePolicyFixtureCompilesAndOrders(t *testing.T) {
	pol, err := policy.Load("../policy/testdata/headroom-builder.policy.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	r := mustRouter(t, mustCompile(t, pol, realCatalog(t), profiles.NewFromDir(profileDir)), cliroute.Host{LookPath: everyBinary(), WeekUsed: weekUsed(t, map[string]float64{"claude": 14})})
	requireChain(t, mustResolve(t, r, cliroute.Request{Agent: "builder", Phase: "build", ProjectRoot: headroomRoot, DefaultModel: "balanced"}), "claude", "agy", "agy-claude")
}

func TestHeadroom_AFamilyOutsideTheUnmeasuredOrderKeepsItsPolicyPlaceAfterIt(t *testing.T) {
	pol := operatorTable()
	pol.CLIRouting.CLIs = append(pol.CLIRouting.CLIs, "codex")
	pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": {CLI: []string{"codex", "agy", "claude"}, Model: "balanced", Order: cliroute.OrderHeadroom}}
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	r := mustRouter(t, mustCompile(t, pol, realCatalog(t), profiles.NewFromDir(profileDir)), cliroute.Host{LookPath: everyBinary(), WeekUsed: weekUsed(t, map[string]float64{"claude": 14})})
	d := mustResolve(t, r, cliroute.Request{Agent: "builder", Phase: "build", ProjectRoot: headroomRoot, DefaultModel: "balanced"})
	if got := families(d.Plan.Candidates)[:3]; !reflect.DeepEqual(got, []string{"claude", "agy", "codex"}) {
		t.Fatalf("chain head = %v, want claude, agy, codex", got)
	}
	requireTrace(t, d, "codex week unknown")
}

func TestHeadroom_AnEnvOverrideStillLeadsTheHeadroomOrder(t *testing.T) {
	r := mustRouter(t, headroomTable(t, headroomRule()), cliroute.Host{LookPath: everyBinary(), WeekUsed: weekUsed(t, map[string]float64{"claude": 14})})
	d := mustResolve(t, r, cliroute.Request{Agent: "builder", Phase: "build", ProjectRoot: headroomRoot, DefaultModel: "balanced", Env: map[string]string{"EVOLVE_BUILDER_CLI": "agy"}})
	if got := families(d.Plan.Candidates); got[0] != "agy" {
		t.Fatalf("an env override must lead after the headroom order, got %v", got)
	}
}

func TestCompile_AnOrderedRuleThatCanLeadWithItsPartnersFamilyWarnsTheCrossFamilyPair(t *testing.T) {
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	pol := operatorTable()
	pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": headroomRule()}
	_, findings := cliroute.Compile(pol, realCatalog(t), profiles.NewFromDir(profileDir))
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("the ordered builder must compile: %+v", errs)
	}
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn, "order headroom", "can start on claude")
	requireFinding(t, findings, "cross_family_with.builder+tdd-engineer", cliroute.SeverityWarn, "order headroom", "can start on claude")
	pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": {CLI: []string{"claude", "agy"}, Model: "balanced", Order: cliroute.OrderHeadroom}}
	_, findings = cliroute.Compile(pol, realCatalog(t), profiles.NewFromDir(profileDir))
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("an ordered rule replaces the static first-family error with the warning: %+v", errs)
	}
	plain := operatorTable()
	plain.CLIRouting.Agents = map[string]policy.AgentRule{"builder": {CLI: []string{"claude", "agy"}, Model: "balanced"}}
	_, findings = cliroute.Compile(plain, realCatalog(t), profiles.NewFromDir(profileDir))
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityError, "both start on claude")
}

func TestCompile_AnOrderedRuleWithoutItsPartnersFamilyKeepsTheStaticCheck(t *testing.T) {
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	pol := operatorTable()
	pol.CLIRouting.Agents = map[string]policy.AgentRule{"builder": {CLI: []string{"agy", "agy-claude"}, Model: "balanced", Order: cliroute.OrderHeadroom}}
	_, findings := cliroute.Compile(pol, realCatalog(t), profiles.NewFromDir(profileDir))
	if _, warned := findingFor(findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn, "order headroom"); warned {
		t.Fatalf("a chain without claude cannot lead with claude: %+v", findings)
	}
}
