package cliroute_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

func mustBuild(t *testing.T, s cliroute.Setup) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(s)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func TestBuild_BypassPolicyPicksTheTableWithoutThePins(t *testing.T) {
	none := []string{}
	loaded := policy.Policy{
		Pins:     map[string]policy.Pin{"scout": {CLI: "claude", Model: "deep"}},
		Workflow: &policy.WorkflowPolicy{UniversalFallbackExclude: none},
	}
	discover := func() []string { return []string{"claude-tmux", "agy-tmux"} }
	r := mustBuild(t, cliroute.Setup{Policy: loaded, Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t),
		Host: cliroute.Host{LookPath: everyBinary(), Discover: discover}})
	pinned := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	if pinned.Rule != cliroute.RuleLegacyPin || !reflect.DeepEqual(pinned.Plan.Candidates, []string{"claude-tmux"}) {
		t.Fatalf("the declared table honours the pin: %+v", pinned)
	}
	bypassed := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", BypassPolicy: true})
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if bypassed.Rule == cliroute.RuleLegacyPin || !reflect.DeepEqual(bypassed.Plan.Candidates, want) {
		t.Fatalf("bypass ignores the pin but keeps the workflow tail: got %+v, want chain %v", bypassed, want)
	}
}

func TestBuild_RefusesAnErrorFindingAndReturnsEveryFinding(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}}
	r, findings, err := cliroute.Build(cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t)})
	if r != nil || err == nil || !strings.Contains(err.Error(), "cli_routing.clis") {
		t.Fatalf("a table missing claude must not route: router=%v err=%v", r, err)
	}
	requireFinding(t, findings, "cli_routing.clis", cliroute.SeverityError, "claude must be listed")
}

func TestBuild_ReturnsTheWarningsOfARoutableTable(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}
	r, findings, err := cliroute.Build(cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t)})
	if err != nil || r == nil {
		t.Fatalf("a Claude-only table routes: %v", err)
	}
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn)
	if !reflect.DeepEqual(r.Findings(), findings) {
		t.Fatalf("Router.Findings = %+v, want the Build findings %+v", r.Findings(), findings)
	}
}

func TestRouter_PolicyIsTheCompiledPolicy(t *testing.T) {
	loaded := policy.Policy{Pins: map[string]policy.Pin{"memo": {Model: "fast"}}}
	r := mustBuild(t, cliroute.Setup{Policy: loaded, Profiles: syntheticProfiles(t)})
	if got := r.Policy(); !reflect.DeepEqual(got.Pins, loaded.Pins) {
		t.Fatalf("Policy() = %+v, want the policy the router was built from", got)
	}
}

func writeProfile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRouter_RecompileSeesAProfileMintedAfterBuild(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "auditor", `{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"]}`)
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: profiles.NewFromDir(dir), Host: cliroute.Host{LookPath: everyBinary()}})
	before := mustResolve(t, r, cliroute.Request{Agent: "minted"})
	if before.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("an unseen agent has no profile restriction: %v", before.Plan.Candidates)
	}
	writeProfile(t, dir, "minted", `{"name":"minted","cli":"claude-tmux","allowed_clis":["claude"]}`)
	if err := r.Recompile(syntheticCatalog()); err != nil {
		t.Fatalf("Recompile: %v", err)
	}
	after := mustResolve(t, r, cliroute.Request{Agent: "minted"})
	if !reflect.DeepEqual(after.Plan.Candidates, []string{"claude-tmux"}) {
		t.Fatalf("the minted profile's allowed_clis must bind after Recompile: %v", after.Plan.Candidates)
	}
}

func TestRouter_ARecompileWithAnErrorFindingKeepsTheRoutingTable(t *testing.T) {
	dir := t.TempDir()
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy"}, AfterChain: "stop"}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: profiles.NewFromDir(dir), Host: cliroute.Host{LookPath: everyBinary()}})
	writeProfile(t, dir, "minted", `{"name":"minted","cli":"codex-tmux","allowed_clis":["codex"]}`)
	err := r.Recompile(syntheticCatalog())
	if err == nil || !strings.Contains(err.Error(), "agent.minted") {
		t.Fatalf("a mint the table cannot route is refused: %v", err)
	}
	if d := mustResolve(t, r, cliroute.Request{Agent: "minted"}); d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("the refused recompile keeps the table compiled at start: %v", d.Plan.Candidates)
	}
}

func TestRouter_ARouterBuiltFromOneTableCannotRecompile(t *testing.T) {
	table, _ := cliroute.Compile(policy.Policy{}, syntheticCatalog(), syntheticProfiles(t))
	if err := mustRouter(t, table, cliroute.Host{}).Recompile(syntheticCatalog()); err == nil {
		t.Fatal("a router with no Setup has nothing to recompile from")
	}
}

func TestResolveRole_TheLegacyProjectionIsTodaysProfileRole(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, dir, "failure-advisor", `{"name":"failure-advisor","cli":"codex","model_tier_default":"fast"}`)
	r := mustBuild(t, cliroute.Setup{Profiles: profiles.NewFromDir(dir)})
	got, err := r.ResolveRole("failure-advisor", resolvellm.Options{ProjectRoot: root, GitRoot: root})
	want := resolvellm.Result{CLI: "codex", ModelTier: "fast", Source: "profile"}
	if err != nil || got != want {
		t.Fatalf("ResolveRole = %+v, %v; want resolvellm's own %+v", got, err, want)
	}
	if _, err := r.ResolveRole("nobody", resolvellm.Options{ProjectRoot: root, GitRoot: root}); !errors.Is(err, resolvellm.ErrProfileNotFound) {
		t.Fatalf("an unknown role keeps resolvellm's error, got %v", err)
	}
}

func TestResolveRole_ADeclaredTableAnswersFromTheTable(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	got, err := r.ResolveRole("scout", resolvellm.Options{})
	want := resolvellm.Result{CLI: "agy-tmux", ModelTier: "balanced", Source: "default"}
	if err != nil || got != want {
		t.Fatalf("ResolveRole = %+v, %v; want %+v", got, err, want)
	}
	if _, err := r.ResolveRole("memo", resolvellm.Options{}); err != nil {
		t.Fatalf("memo resolves: %v", err)
	}
}

func TestResolveRole_ADeclaredRefusalIsAnError(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	t.Setenv("EVOLVE_CLI", "codex-tmux")
	if _, err := r.ResolveRole("scout", resolvellm.Options{}); err == nil || !strings.Contains(err.Error(), "outside the allowed set") {
		t.Fatalf("an env primary outside the table is refused: %v", err)
	}
}

func routerProfiles(t *testing.T, body string) cliroute.ProfileSource {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, "router", body)
	return profiles.NewFromDir(dir)
}

func TestResolve_TheLegacyAdvisorProjectionIsTodaysRouterDispatch(t *testing.T) {
	routerKeys := policy.Policy{Router: &policy.RouterPolicy{CLI: "codex-tmux", Model: "balanced"}}
	cases := []struct {
		name  string
		pol   policy.Policy
		profs cliroute.ProfileSource
		chain []string
		model string
		rule  string
	}{
		{"profile", policy.Policy{}, routerProfiles(t, `{"name":"router","cli":"agy-tmux","model_tier_default":"deep"}`), []string{"agy-tmux", "claude-tmux"}, "deep", "legacy:profile"},
		{"router keys", routerKeys, routerProfiles(t, `{"name":"router","cli":"agy-tmux","model_tier_default":"deep"}`), []string{"codex-tmux", "claude-tmux"}, "balanced", "legacy:router"},
		{"no profile", policy.Policy{}, routerProfiles(t, `{"name":"other"}`), []string{"claude-tmux"}, "opus", "legacy:default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := mustBuild(t, cliroute.Setup{Policy: tc.pol, Profiles: tc.profs})
			t.Setenv("EVOLVE_CLI", "claude-p")
			d := mustResolve(t, r, cliroute.Request{Agent: "router", Launch: cliroute.LaunchAdvisor, DefaultModel: "opus"})
			if !reflect.DeepEqual(d.Plan.Candidates, tc.chain) || d.Plan.Model != tc.model || d.Rule != tc.rule {
				t.Fatalf("got chain=%v model=%s rule=%s; want %v %s %s (env never reaches the advisor)", d.Plan.Candidates, d.Plan.Model, d.Rule, tc.chain, tc.model, tc.rule)
			}
		})
	}
}

func TestResolve_ADeclaredAdvisorFollowsTheTable(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: routerProfiles(t, `{"name":"router","cli":"codex-tmux","model_tier_default":"deep"}`),
		Host: cliroute.Host{LookPath: everyBinary()}})
	d := mustResolve(t, r, cliroute.Request{Agent: "router", Launch: cliroute.LaunchAdvisor, DefaultModel: "opus"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"agy-tmux", "claude-tmux"}) || d.Rule != "default" || d.Plan.Model != "deep" {
		t.Fatalf("the declared advisor takes the table's chain: %+v", d)
	}
}

func TestResolve_TheLegacyClassifierKeepsTodaysOrder(t *testing.T) {
	r := mustBuild(t, cliroute.Setup{Profiles: syntheticProfiles(t)})
	d := mustResolve(t, r, cliroute.Request{Agent: cliroute.ClassifierAgent, Launch: cliroute.LaunchClassifier})
	if got := families(d.Plan.Candidates); !reflect.DeepEqual(got, []string{"codex", "claude", "agy"}) || d.Rule != "legacy:classifier" {
		t.Fatalf("the legacy classifier order is codex > claude > agy: %v (%s)", got, d.Rule)
	}
}

func TestResolve_ADeclaredClassifierNeverLaunchesAnUnlistedFamily(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	d := mustResolve(t, r, cliroute.Request{Agent: cliroute.ClassifierAgent, Launch: cliroute.LaunchClassifier})
	if got := families(d.Plan.Candidates); !reflect.DeepEqual(got, []string{"agy", "claude"}) {
		t.Fatalf("clis [agy, claude] never classify on codex: %v", got)
	}
}

func TestResolve_ALegacyModelOnlyPinIsThePinRule(t *testing.T) {
	r := mustBuild(t, cliroute.Setup{Policy: policy.Policy{Pins: map[string]policy.Pin{"memo": {Model: "fast"}}}, Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	if d := mustResolve(t, r, cliroute.Request{Agent: "memo", Phase: "memo"}); d.Rule != cliroute.RuleLegacyPin {
		t.Fatalf("a model-only pin still decides the dispatch: rule=%s", d.Rule)
	}
}

func TestResolve_TheLegacyProjectionLogsAReorderedChain(t *testing.T) {
	var lines []string
	discover := func() []string { return []string{"claude-tmux", "agy-tmux"} }
	r := mustBuild(t, cliroute.Setup{Policy: policy.Policy{Workflow: &policy.WorkflowPolicy{UniversalFallbackExclude: []string{}}}, Profiles: syntheticProfiles(t),
		Host: cliroute.Host{LookPath: installed("claude", "agy"), Discover: discover, Logf: func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }}})
	mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"})
	joined := strings.Join(lines, "|")
	if !strings.Contains(joined, "capability probe reordered") || !strings.Contains(joined, "universal fallback appended") {
		t.Fatalf("the probe and the tail each log a reorder: %q", lines)
	}
}

func TestSingleProfile_ListsAndGetsOnlyItsAgent(t *testing.T) {
	prof := &profiles.Profile{Name: "builder", CLI: "codex-tmux"}
	src := cliroute.SingleProfile{Agent: "builder", Profile: prof}
	if names, err := src.List(); err != nil || !reflect.DeepEqual(names, []string{"builder"}) {
		t.Fatalf("List = %v, %v", names, err)
	}
	if got, err := src.Get("builder"); err != nil || got.CLI != "codex-tmux" {
		t.Fatalf("Get(builder) = %+v, %v", got, err)
	}
	if _, err := src.Get("auditor"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("another agent is not in the source: %v", err)
	}
	if names, err := (cliroute.SingleProfile{Agent: "x"}).List(); err != nil || len(names) != 0 {
		t.Fatalf("no profile lists nothing: %v %v", names, err)
	}
}

func TestSingleProfile_AnAbsentProfileRoutesAsToday(t *testing.T) {
	table, findings := cliroute.Compile(policy.Policy{}, nil, cliroute.SingleProfile{Agent: "retrospective"})
	if len(findings) != 0 {
		t.Fatalf("an absent profile is no finding: %+v", findings)
	}
	d := mustResolve(t, mustRouter(t, table, cliroute.Host{}), cliroute.Request{Agent: "retrospective"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"claude-tmux"}) || d.Plan.Candidates[0] != llmroute.DefaultDriverForFamily("claude") {
		t.Fatalf("no profile routes to the claude default: %v", d.Plan.Candidates)
	}
}

type profilesListingOnce struct {
	cliroute.ProfileSource
	listed int
}

func (p *profilesListingOnce) List() ([]string, error) {
	p.listed++
	if p.listed > 1 {
		return nil, errors.New("the profiles directory went away")
	}
	return p.ProfileSource.List()
}

func TestBuild_ABypassTableThatCannotListTheProfilesIsRefused(t *testing.T) {
	_, _, err := cliroute.Build(cliroute.Setup{Profiles: &profilesListingOnce{ProfileSource: syntheticProfiles(t)}})
	if err == nil || !strings.Contains(err.Error(), "the profiles directory went away") {
		t.Fatalf("the bypass table is held to the same refusal: %v", err)
	}
}

func TestResolve_ADeclaredRefusalIsErrRefusedAndALegacyPinErrorIsNot(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	if _, err := r.Resolve(cliroute.Request{Agent: "scout", Env: map[string]string{"EVOLVE_SCOUT_CLI": "codex-tmux"}}); !errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("a declared refusal carries ErrRefused: %v", err)
	}
	legacy := mustBuild(t, cliroute.Setup{Policy: policy.Policy{Pins: map[string]policy.Pin{"audit": {CLI: "agy"}}}, Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t)})
	if _, err := legacy.Resolve(cliroute.Request{Agent: "auditor", Phase: "audit"}); err == nil || errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("the legacy pin error keeps the runner's own text, unwrapped: %v", err)
	}
}

func TestDecision_LegacyNamesTheLegacyProjection(t *testing.T) {
	legacy := mustResolve(t, mustBuild(t, cliroute.Setup{Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}}), cliroute.Request{Agent: "scout", Phase: "scout"})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	declared := mustResolve(t, mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Catalog: syntheticCatalog(), Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}}), cliroute.Request{Agent: "scout", Phase: "scout"})
	if !legacy.Legacy() || declared.Legacy() {
		t.Fatalf("Legacy() = %v for %s, %v for %s", legacy.Legacy(), legacy.Rule, declared.Legacy(), declared.Rule)
	}
}

func TestNewSingleProfileRouter_RoutesExactlyLikeCompilingAndBuildingByHand(t *testing.T) {
	prof := &profiles.Profile{Name: "builder", CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}}
	src := cliroute.SingleProfile{Agent: "builder", Profile: prof}
	host := cliroute.Host{LookPath: everyBinary()}
	req := cliroute.Request{Agent: "builder", Phase: "build"}

	got, err := cliroute.NewSingleProfileRouter(policy.Policy{}, src, host)
	if err != nil {
		t.Fatalf("NewSingleProfileRouter: %v", err)
	}
	table, _ := cliroute.Compile(policy.Policy{}, nil, src)
	want := mustRouter(t, table, host)

	gotDec, gotErr := got.Resolve(req)
	wantDec, wantErr := want.Resolve(req)
	if (gotErr == nil) != (wantErr == nil) || !reflect.DeepEqual(gotDec.Plan, wantDec.Plan) || gotDec.Rule != wantDec.Rule {
		t.Fatalf("the helper must route like Compile then New:\n got %+v (%v)\nwant %+v (%v)", gotDec.Plan, gotErr, wantDec.Plan, wantErr)
	}
	if len(gotDec.Plan.Candidates) == 0 || gotDec.Plan.Candidates[0] != "codex-tmux" {
		t.Fatalf("a single-profile router leads with the profile's own CLI: %v", gotDec.Plan.Candidates)
	}
}

func TestCompile_ALegacyTableTreatsAMissingProfilesDirectoryAsEmpty(t *testing.T) {
	missing := profiles.NewFromDir(filepath.Join(t.TempDir(), "no-such-profiles"))
	table, findings := cliroute.Compile(policy.Policy{}, nil, missing)
	if len(errorFindings(findings)) != 0 {
		t.Fatalf("with no cli_routing block a missing profiles directory routes as before: %+v", findings)
	}
	requireFinding(t, findings, "profiles", cliroute.SeverityWarn, "does not exist")
	d := mustResolve(t, mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}), cliroute.Request{Agent: "scout", Phase: "scout"})
	if d.Plan.Candidates[0] != "claude-tmux" {
		t.Fatalf("an agent with no profile takes the claude default, as the runner did: %v", d.Plan.Candidates)
	}
}

func TestCompile_ADeclaredTableStillRefusesAMissingProfilesDirectory(t *testing.T) {
	missing := profiles.NewFromDir(filepath.Join(t.TempDir(), "no-such-profiles"))
	block := policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}
	_, findings := cliroute.Compile(routingPolicy(block), nil, missing)
	requireFinding(t, findings, "profiles", cliroute.SeverityError, "listing the profiles failed")
}

type gatedProfiles struct {
	cliroute.ProfileSource
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

func (g *gatedProfiles) List() ([]string, error) {
	g.mu.Lock()
	g.calls++
	first := g.calls == 3
	g.mu.Unlock()
	if first {
		close(g.entered)
		<-g.release
	}
	return g.ProfileSource.List()
}

func TestRouter_ConcurrentRecompilesKeepTheLastCallersCatalog(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"claude"}, Work: map[string][]string{"plan": {"agy", "claude"}}}
	src := &gatedProfiles{ProfileSource: cliroute.SingleProfile{}, entered: make(chan struct{}), release: make(chan struct{})}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: src, Host: cliroute.Host{LookPath: everyBinary()}})
	older := fakeCatalog{"x": {Name: "x", Role: string(phasespec.RoleBuild)}}
	newer := fakeCatalog{"x": {Name: "x", Role: string(phasespec.RolePlan)}}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = r.Recompile(older) }()
	<-src.entered
	newerDone := make(chan struct{})
	go func() { defer wg.Done(); _ = r.Recompile(newer); close(newerDone) }()
	select {
	case <-newerDone:
	case <-time.After(200 * time.Millisecond):
	}
	close(src.release)
	wg.Wait()
	if d := mustResolve(t, r, cliroute.Request{Agent: "worker", Phase: "x"}); d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("the recompile called last must win: an older catalog stored after a newer one routes x as build (%v, rule %s)", d.Plan.Candidates, d.Rule)
	}
}

func TestCompile_ThePerDecisionRouterModelsAreASecondSourceBesideTheBlock(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}
	p := policy.Policy{CLIRouting: &block, Router: &policy.RouterPolicy{PlanModel: "top", ProposeModel: "fast"}}
	_, findings := cliroute.Compile(p, syntheticCatalog(), syntheticProfiles(t))
	requireFinding(t, findings, "router.plan_model", cliroute.SeverityError, "cli-routing migrate")
	requireFinding(t, findings, "router.propose_model", cliroute.SeverityError, "cli-routing migrate")
	if _, legacy := cliroute.Compile(policy.Policy{Router: &policy.RouterPolicy{PlanModel: "top"}}, syntheticCatalog(), syntheticProfiles(t)); len(legacy) != 0 {
		t.Fatalf("with no block the per-decision models stay the advisor's legacy keys: %+v", legacy)
	}
}
