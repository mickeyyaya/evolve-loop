package cliroute_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func syntheticRouter(t *testing.T, block policy.CLIRouting, host cliroute.Host) *cliroute.Router {
	t.Helper()
	if host.LookPath == nil {
		host.LookPath = everyBinary()
	}
	return mustRouter(t, mustCompile(t, routingPolicy(block), syntheticCatalog(), syntheticProfiles(t)), host)
}

func mustResolve(t *testing.T, r *cliroute.Router, req cliroute.Request) cliroute.Decision {
	t.Helper()
	d, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve(%+v): %v", req, err)
	}
	return d
}

var agyClaude = []string{"agy", "claude"}

func TestResolve_PrecedenceIsAgentsThenWorkThenDefaultThenProfile(t *testing.T) {
	base := policy.CLIRouting{CLIs: []string{"agy", "claude", "codex"}, AfterChain: "stop"}
	cases := []struct {
		name  string
		block func(policy.CLIRouting) policy.CLIRouting
		rule  string
		chain []string
	}{
		{"profile", func(b policy.CLIRouting) policy.CLIRouting { return b }, "profile", []string{"codex-tmux", "claude-tmux"}},
		{"default", func(b policy.CLIRouting) policy.CLIRouting { b.Default = agyClaude; return b }, "default", []string{"agy-tmux", "claude-tmux"}},
		{"work", func(b policy.CLIRouting) policy.CLIRouting {
			b.Default = agyClaude
			b.Work = map[string][]string{"plan": {"claude"}}
			return b
		}, "work:plan", []string{"claude-tmux"}},
		{"agents", func(b policy.CLIRouting) policy.CLIRouting {
			b.Default = agyClaude
			b.Work = map[string][]string{"plan": {"claude"}}
			b.Agents = map[string]policy.AgentRule{"scout": {CLI: []string{"codex", "agy"}}}
			return b
		}, "agents:scout", []string{"codex-tmux", "agy-tmux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := mustResolve(t, syntheticRouter(t, tc.block(base), cliroute.Host{}), cliroute.Request{Agent: "scout", Phase: "scout"})
			if d.Rule != tc.rule || !reflect.DeepEqual(d.Plan.Candidates, tc.chain) {
				t.Fatalf("rule=%s chain=%v, want %s %v", d.Rule, d.Plan.Candidates, tc.rule, tc.chain)
			}
			if d.Plan.PrimarySource != tc.rule {
				t.Fatalf("PrimarySource = %q, want the rule %q", d.Plan.PrimarySource, tc.rule)
			}
		})
	}
}

func TestResolve_TheWorkRoleComesFromThePhase(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude, Work: map[string][]string{"plan": {"claude"}}}, cliroute.Host{})
	if d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"}); d.Rule != "work:plan" {
		t.Fatalf("scout's phase is a plan phase, got rule %s", d.Rule)
	}
	if d := mustResolve(t, r, cliroute.Request{Agent: "memo", Phase: "memo"}); d.Rule != "default" {
		t.Fatalf("memo's phase is a control phase with no work rule, got rule %s", d.Rule)
	}
}

func TestResolve_APhaselessLaunchTakesTheAgentsOnlyRole(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: []string{"agy"}, Work: map[string][]string{"evaluate": {"claude"}}, Agents: map[string]policy.AgentRule{"tdd-engineer": {CLI: []string{"claude"}}, "spec-verify": {CLI: []string{"claude"}}}}, cliroute.Host{})
	phased := mustResolve(t, r, cliroute.Request{Agent: "auditor", Phase: "audit"})
	phaseless := mustResolve(t, r, cliroute.Request{Agent: "auditor"})
	if phaseless.Rule != "work:evaluate" || !reflect.DeepEqual(phaseless.Plan.Candidates, phased.Plan.Candidates) {
		t.Fatalf("the auditor serves only an evaluate phase, so a phase-less launch takes that role: %s %v vs %s %v",
			phaseless.Rule, phaseless.Plan.Candidates, phased.Rule, phased.Plan.Candidates)
	}
}

func TestResolve_AnAgentWithNoPhaseOrSeveralRolesHasNoRoleWhenThePhaseIsEmpty(t *testing.T) {
	cat := syntheticCatalog()
	cat["review"] = phasespec.PhaseSpec{Name: "review", Agent: "evolve-scout", Role: "evaluate"}
	table := mustCompile(t, routingPolicy(policy.CLIRouting{CLIs: agyClaude, Default: agyClaude, Work: map[string][]string{"plan": {"claude"}}}), cat, syntheticProfiles(t))
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})
	if d := mustResolve(t, r, cliroute.Request{Agent: "scout"}); d.Rule != "default" {
		t.Fatalf("scout serves a plan and an evaluate phase, so a phase-less launch has no role: rule %s", d.Rule)
	}
	if d := mustResolve(t, r, cliroute.Request{Agent: "spec-verify"}); d.Rule != "default" {
		t.Fatalf("spec-verify serves no phase, so a phase-less launch has no role: rule %s", d.Rule)
	}
}

func TestResolve_AnAgentKeyedByItsPhaseAppliesToThatAgent(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude", "codex"}, Default: agyClaude, AfterChain: "stop",
		Agents: map[string]policy.AgentRule{"build": {CLI: []string{"codex"}}},
	}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "builder", Phase: "build"})
	if d.Rule != "agents:builder" || !reflect.DeepEqual(d.Plan.Candidates, []string{"codex-tmux"}) {
		t.Fatalf("agents.build must reach the builder: rule=%s chain=%v", d.Rule, d.Plan.Candidates)
	}
}

func TestResolve_ADefaultChainIsFilteredSilentlyToTheAllowedSet(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "auditor", Phase: "audit"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux"}) || !reflect.DeepEqual(d.Allowed, []string{"claude"}) {
		t.Fatalf("the floor agent keeps only claude: chain=%v allowed=%v", d.Plan.Candidates, d.Allowed)
	}
	if !strings.Contains(strings.Join(d.Trace, "\n"), "agy-tmux") {
		t.Fatalf("the filtering is traced, got %v", d.Trace)
	}
}

func TestResolve_NoDefaultFallsBackToTheProfileChainFilteredByClis(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Agents: map[string]policy.AgentRule{"builder": {CLI: agyClaude}}}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	if d.Rule != "profile" || !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux", "agy-tmux"}) {
		t.Fatalf("profile chain [codex claude] minus codex, then after_chain agy: rule=%s chain=%v", d.Rule, d.Plan.Candidates)
	}
}

func TestResolve_AfterChainOtherClisAppendsTheRemainingClisInTheirOrder(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: []string{"codex", "agy", "claude"}, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"scout": {CLI: []string{"claude"}}},
	}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	if want := []string{"claude-tmux", "codex-tmux", "agy-tmux"}; !reflect.DeepEqual(d.Plan.Candidates, want) {
		t.Fatalf("chain = %v, want %v", d.Plan.Candidates, want)
	}
}

func TestResolve_AfterChainStopEndsTheWalk(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude, AfterChain: "stop",
		Agents: map[string]policy.AgentRule{"scout": {CLI: []string{"agy"}}},
	}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"agy-tmux"}) {
		t.Fatalf("stop appends nothing: %v", d.Plan.Candidates)
	}
	res := walkWith(d.Plan, map[string]int{"agy-tmux@balanced": 85})
	if !reflect.DeepEqual(res.Attempts, []string{"agy-tmux@balanced"}) || res.Err == nil {
		t.Fatalf("the walk ends after the chain: %+v", res)
	}
}

func TestResolve_AnAssignmentKeepsTheProbeAndTheBench(t *testing.T) {
	benched := func(_, _ string, p llmroute.Plan, _ map[string]string) llmroute.Plan {
		return llmroute.ApplyBench(p, map[string]time.Time{"agy": time.Unix(0, 0)})
	}
	block := policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}
	d := mustResolve(t, syntheticRouter(t, block, cliroute.Host{Bench: benched}), cliroute.Request{Agent: "scout", Phase: "scout"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux", "agy-tmux"}) {
		t.Fatalf("a benched agy is demoted, not locked in: %v", d.Plan.Candidates)
	}
	d = mustResolve(t, syntheticRouter(t, block, cliroute.Host{LookPath: installed("claude")}), cliroute.Request{Agent: "scout", Phase: "scout"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux", "agy-tmux"}) {
		t.Fatalf("a missing agy binary is demoted by the probe: %v", d.Plan.Candidates)
	}
}

func TestResolve_TheAdvisorRaisesTheTierButCannotChangeTheCLI(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Overlay: llmroute.Overlay{CLI: "claude", Tier: "deep"}})
	if d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("the table's chain outranks the advisor's CLI: %v", d.Plan.Candidates)
	}
	if d.Plan.Model != "deep" || !reflect.DeepEqual(d.Plan.Tiers, []string{"deep", "balanced"}) {
		t.Fatalf("the advisor still raises the tier: model=%s tiers=%v", d.Plan.Model, d.Plan.Tiers)
	}
}

func TestResolve_AnAgentModelOutranksTheAdvisorTier(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"scout": {CLI: []string{"agy"}, Model: "fast"}},
	}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Overlay: llmroute.Overlay{Tier: "deep"}})
	if d.Plan.Model != "fast" || d.Plan.Tiers[0] != "fast" {
		t.Fatalf("agents.scout.model fixes the tier: model=%s tiers=%v", d.Plan.Model, d.Plan.Tiers)
	}
	d = mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Env: map[string]string{"EVOLVE_SCOUT_MODEL": "top"}})
	if d.Plan.Model != "top" {
		t.Fatalf("a per-run model env stays a per-run override: %s", d.Plan.Model)
	}
}

func TestResolve_TheAdvisorCLIAppliesUnderTheProfileRuleWithinTheAllowedSet(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Agents: map[string]policy.AgentRule{"builder": {CLI: agyClaude}}}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Overlay: llmroute.Overlay{CLI: "agy"}})
	if d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("no table rule covers scout, so the advisor's CLI is promoted: %v", d.Plan.Candidates)
	}
	d = mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Overlay: llmroute.Overlay{CLI: "codex"}})
	if families(d.Plan.Candidates)[0] == "codex" {
		t.Fatalf("an advisor CLI outside the allowed set is never promoted: %v", d.Plan.Candidates)
	}
}

func TestResolve_EnvOverrideOutsideAllowedFails(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{})
	cases := []struct {
		agent, phase, key, value string
	}{
		{"auditor", "audit", "EVOLVE_AUDITOR_CLI", "agy-tmux"},
		{"scout", "scout", "EVOLVE_CLI", "codex-tmux"},
		{"scout", "scout", "EVOLVE_SCOUT_CLI", "codex"},
	}
	for _, tc := range cases {
		_, err := r.Resolve(cliroute.Request{Agent: tc.agent, Phase: tc.phase, Env: map[string]string{tc.key: tc.value}})
		if err == nil || !strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), "allowed") {
			t.Errorf("%s=%s for %s: err = %v, want a refusal naming the key", tc.key, tc.value, tc.agent, err)
		}
	}
	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Env: map[string]string{"EVOLVE_SCOUT_CLI": "claude-p"}})
	if d.Plan.Candidates[0] != "claude-p" || d.Plan.PrimarySource != "env(EVOLVE_SCOUT_CLI)" {
		t.Fatalf("an allowed env primary leads the chain: %v from %s", d.Plan.Candidates, d.Plan.PrimarySource)
	}
}

func TestResolve_ACallerCLIAppliesOnlyWhenNoTableRuleCoversTheAgent(t *testing.T) {
	ruled := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{})
	if d := mustResolve(t, ruled, cliroute.Request{Agent: "scout", CallerCLI: "claude-p"}); d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("a table rule outranks the caller's CLI: %v", d.Plan.Candidates)
	}
	unruled := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Agents: map[string]policy.AgentRule{"builder": {CLI: agyClaude}}}, cliroute.Host{})
	if d := mustResolve(t, unruled, cliroute.Request{Agent: "scout", CallerCLI: "claude-p"}); d.Plan.Candidates[0] != "claude-p" {
		t.Fatalf("with no rule the caller's CLI leads: %v", d.Plan.Candidates)
	}
	d := mustResolve(t, unruled, cliroute.Request{Agent: "auditor", CallerCLI: "agy-tmux"})
	if families(d.Plan.Candidates)[0] != "claude" || !strings.Contains(strings.Join(d.Trace, "\n"), "agy-tmux") {
		t.Fatalf("a caller CLI outside the allowed set is ignored and traced: %v %v", d.Plan.Candidates, d.Trace)
	}
}

func TestResolve_APerRunTierTheCeilingEmptiesIsATypedCeilingError(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude, AfterChain: "stop",
		Agents: map[string]policy.AgentRule{"memo": {CLI: []string{"agy"}}},
		Tiers:  map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "balanced": {CLIs: []string{"claude"}}},
	}, cliroute.Host{})
	mustResolve(t, r, cliroute.Request{Agent: "memo", Phase: "memo"})
	_, err := r.Resolve(cliroute.Request{Agent: "memo", Phase: "memo", Env: map[string]string{"EVOLVE_MEMO_MODEL": "deep"}})
	var ceilingErr *cliroute.CeilingError
	if !errors.As(err, &ceilingErr) || !strings.Contains(err.Error(), "tier ceiling") {
		t.Fatalf("an agy-only chain run at claude-only tiers has nothing to launch: %v", err)
	}
	if ceilingErr.Rule != "agents:memo" || !reflect.DeepEqual(ceilingErr.Tiers, []string{"deep", "balanced"}) ||
		!reflect.DeepEqual(ceilingErr.Chain, []string{"agy-tmux"}) || !reflect.DeepEqual(ceilingErr.Ceiling["deep"], []string{"claude"}) {
		t.Fatalf("CeilingError = %+v", ceilingErr)
	}
}

func TestResolve_ARuleThatLeavesNothingIsATypedRuleError(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: []string{"claude"}}, cliroute.Host{})
	_, err := r.Resolve(cliroute.Request{Agent: "nobody"})
	var ruleErr *cliroute.RuleError
	if !errors.As(err, &ruleErr) || ruleErr.Kind != cliroute.RuleEmpty || ruleErr.Rule != "profile" || !strings.Contains(err.Error(), "nobody") {
		t.Fatalf("no rule and no profile chain is a typed empty-chain error naming the agent: %v", err)
	}
	if !reflect.DeepEqual(ruleErr.Allowed, []string{"claude"}) || ruleErr.Dropped != nil {
		t.Fatalf("RuleError = %+v", ruleErr)
	}
}

func TestResolve_ReadsOnlyTheProfilesCompileSnapshotted(t *testing.T) {
	fsys := fstest.MapFS{"scout.json": {Data: []byte(`{"name":"scout","cli":"agy-tmux","allowed_clis":["claude","agy"]}`)}}
	counting := &countingProfiles{ProfileSource: profiles.NewFromFS(fsys)}
	table := mustCompile(t, routingPolicy(policy.CLIRouting{CLIs: agyClaude, AfterChain: "stop"}), syntheticCatalog(), counting)
	reads := counting.gets
	fsys["scout.json"] = &fstest.MapFile{Data: []byte(`{"name":"scout","cli":"claude-tmux","allowed_clis":["claude"]}`)}
	d := mustResolve(t, mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}), cliroute.Request{Agent: "scout", Phase: "scout"})
	if counting.gets != reads || !reflect.DeepEqual(d.Plan.Candidates, []string{"agy-tmux"}) {
		t.Fatalf("Resolve re-read the profiles (%d -> %d reads) or missed the snapshot: %v", reads, counting.gets, d.Plan.Candidates)
	}
}

type countingProfiles struct {
	cliroute.ProfileSource
	gets int
}

func (c *countingProfiles) Get(name string) (profiles.Profile, error) {
	c.gets++
	return c.ProfileSource.Get(name)
}

func TestResolve_TheFloorHoldsEvenWhenTheProfileAllowsEveryFamily(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: []string{"claude"}, Agents: map[string]policy.AgentRule{"builder": {CLI: agyClaude}}}, cliroute.Host{})
	d := mustResolve(t, r, cliroute.Request{Agent: "spec-verify", Phase: "spec-verify"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux"}) || !reflect.DeepEqual(d.Allowed, []string{"claude"}) {
		t.Fatalf("after_chain must not hand a floor agent to agy: %v (allowed %v)", d.Plan.Candidates, d.Allowed)
	}
}

func TestResolve_AZeroTableResolvesTheLegacyDefault(t *testing.T) {
	d := mustResolve(t, mustRouter(t, cliroute.Table{}, cliroute.Host{LookPath: everyBinary()}), cliroute.Request{Agent: "scout", DefaultModel: "balanced"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux"}) || d.Rule != "legacy:default" || d.Allowed != nil {
		t.Fatalf("a zero table has no profiles and no table: %+v", d)
	}
}

func TestResolve_TheHostLogfReceivesEachDecision(t *testing.T) {
	var lines []string
	logf := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{Logf: logf})
	mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	if len(lines) != 1 || !strings.Contains(lines[0], "agent=scout") || !strings.Contains(lines[0], "rule=default") {
		t.Fatalf("one decision line naming the agent and the rule, got %q", lines)
	}
}

func TestNew_RefusesATableWithAnErrorFinding(t *testing.T) {
	table, findings := cliroute.Compile(routingPolicy(policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"auditor": {CLI: agyClaude}},
	}), syntheticCatalog(), syntheticProfiles(t))
	if !reflect.DeepEqual(table.Findings(), findings) {
		t.Fatalf("the table carries its findings: %+v vs %+v", table.Findings(), findings)
	}
	r, err := cliroute.New(table, cliroute.Host{})
	if r != nil || err == nil || !strings.Contains(err.Error(), "cli_routing.agents.auditor") {
		t.Fatalf("New must refuse a table with an error finding and name it: %v %v", r, err)
	}
	warned := mustCompile(t, routingPolicy(policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}), syntheticCatalog(), syntheticProfiles(t))
	if len(warned.Findings()) == 0 {
		t.Fatal("the shared-fallback warning is expected on this table")
	}
	mustRouter(t, warned, cliroute.Host{})
}

func TestDecision_AllowsByFamily(t *testing.T) {
	d := cliroute.Decision{Allowed: agyClaude}
	for cli, want := range map[string]bool{"agy": true, "agy-tmux": true, "claude-p": true, "codex-tmux": false, "codex": false} {
		if got := d.Allows(cli); got != want {
			t.Errorf("Allows(%s) = %v, want %v", cli, got, want)
		}
	}
	if !(cliroute.Decision{}).Allows("codex-tmux") {
		t.Error("a legacy decision with no allowed set restricts nothing")
	}
}

func TestRuleError_NamesTheRuleTheKindAndTheSets(t *testing.T) {
	cases := []struct {
		kind cliroute.RuleErrorKind
		want []string
	}{
		{cliroute.RuleLeak, []string{"agents:auditor lists [agy-tmux] outside the allowed set [claude]"}},
		{cliroute.RuleEmpty, []string{"rule agents:auditor leaves no allowed CLI", "[agy-tmux] dropped", "allowed [claude]"}},
	}
	for _, tc := range cases {
		err := &cliroute.RuleError{Rule: "agents:auditor", Kind: tc.kind, Dropped: []string{"agy-tmux"}, Allowed: []string{"claude"}}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q lacks %q", tc.kind, err.Error(), w)
			}
		}
	}
}

func TestCeilingError_NamesTheRuleTheTiersTheChainAndTheCeiling(t *testing.T) {
	err := &cliroute.CeilingError{Rule: "default", Tiers: []string{"deep"}, Chain: []string{"agy-tmux"}, Ceiling: map[string][]string{"deep": {"claude"}}}
	for _, want := range []string{"rule default", "tier ceiling", "[agy-tmux]", "[deep]", "deep:[claude]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q lacks %q", err.Error(), want)
		}
	}
}
