package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const (
	agyClaudeFamily            = "agy-claude"
	agyClaudeTarget            = "agy-claude-tmux"
	launchModelVerificationKey = "launch_model_verification"
	floorSeatRisk              = "a floor seat would silently run Gemini and break builder != auditor"
	otherSeatRisk              = "a deep or top seat would silently run agy's default Gemini model"
)

func checkedInRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func agyClaudeEntries(where string, chain []string) []string {
	var routes []string
	for _, entry := range chain {
		if policy.BaseCLI(entry) == agyClaudeFamily {
			routes = append(routes, where+" "+entry)
		}
	}
	return routes
}

func floorProfileRoutesToAgyClaude(name string, p profiles.Profile) []string {
	routes := agyClaudeEntries("profile "+name+" cli", []string{p.CLI})
	routes = append(routes, agyClaudeEntries("profile "+name+" allowed_clis", p.AllowedCLIs)...)
	return append(routes, agyClaudeEntries("profile "+name+" cli_fallback", p.CLIFallback)...)
}

func policyRoutesToAgyClaude(pol policy.Policy) []string {
	var routes []string
	for agent, pin := range pol.Pins {
		routes = append(routes, agyClaudeEntries("pin "+agent, []string{pin.CLI})...)
	}
	if pol.CLIRouting == nil {
		return routes
	}
	return append(routes, cliRoutingRoutesToAgyClaude(*pol.CLIRouting)...)
}

func cliRoutingRoutesToAgyClaude(table policy.CLIRouting) []string {
	routes := agyClaudeEntries("cli_routing clis", table.CLIs)
	routes = append(routes, agyClaudeEntries("cli_routing default", table.Default)...)
	for role, chain := range table.Work {
		routes = append(routes, agyClaudeEntries("cli_routing work "+role, chain)...)
	}
	for agent, rule := range table.Agents {
		routes = append(routes, agyClaudeEntries("cli_routing agents "+agent, rule.CLI)...)
	}
	for tier, chain := range table.Tiers {
		routes = append(routes, agyClaudeEntries("cli_routing tiers "+tier, chain)...)
	}
	return routes
}

func realTreeAgyClaudeRoutes(t *testing.T) (floorSeats, otherSeats []string) {
	t.Helper()
	root := checkedInRoot(t)
	loader := profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles"))
	for name := range profiles.ClaudeFamilyFloor() {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("floor profile %s: %v", name, err)
		}
		floorSeats = append(floorSeats, floorProfileRoutesToAgyClaude(name, p)...)
	}
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load policy.json: %v", err)
	}
	for _, route := range policyRoutesToAgyClaude(pol) {
		if routesAFloorSeat(route) {
			floorSeats = append(floorSeats, route)
		} else {
			otherSeats = append(otherSeats, route)
		}
	}
	return floorSeats, otherSeats
}

func routesAFloorSeat(route string) bool {
	for _, prefix := range []string{"pin ", "cli_routing agents "} {
		if rest, ok := strings.CutPrefix(route, prefix); ok {
			agent, _, _ := strings.Cut(rest, " ")
			return profiles.IsClaudeFamilyFloor(agent)
		}
	}
	return false
}

func seatGaps(floorSeats, otherSeats []string, verifies func() (bool, error)) (floorGaps, otherGaps []string, err error) {
	if len(floorSeats) == 0 && len(otherSeats) == 0 {
		return nil, nil, nil
	}
	verified, err := verifies()
	if err != nil || verified {
		return nil, nil, err
	}
	return seatGap(floorSeats, floorSeatRisk), seatGap(otherSeats, otherSeatRisk), nil
}

func seatGap(routes []string, risk string) []string {
	if len(routes) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("routes %q reach %s, whose driver does not verify the booted model before the prompt (%s): %s", slices.Sorted(slices.Values(routes)), agyClaudeTarget, launchModelVerificationKey, risk)}
}

func realTreeSeatGaps(t *testing.T) (floorGaps, otherGaps []string) {
	t.Helper()
	floorSeats, otherSeats := realTreeAgyClaudeRoutes(t)
	floorGaps, otherGaps, err := seatGaps(floorSeats, otherSeats, func() (bool, error) { return gobridge.VerifiesLaunchModel(agyClaudeTarget) })
	if err != nil {
		t.Fatalf("resolve %s: %v", agyClaudeTarget, err)
	}
	return floorGaps, otherGaps
}

func TestAgyClaudeRouting_AFloorSeatNeedsTheLaunchModelCheck(t *testing.T) {
	floorGaps, _ := realTreeSeatGaps(t)
	for _, gap := range floorGaps {
		t.Error(gap)
	}
}

func TestAgyClaudeRouting_ANonFloorSeatNeedsTheLaunchModelCheck(t *testing.T) {
	_, otherGaps := realTreeSeatGaps(t)
	for _, gap := range otherGaps {
		t.Error(gap)
	}
}

func TestSeatGaps_EverySeatWaitsForTheVerifiedLaunch(t *testing.T) {
	floor, other := []string{"pin auditor agy-claude"}, []string{"cli_routing tiers deep agy-claude"}
	verified := func() (bool, error) { return true, nil }
	unverified := func() (bool, error) { return false, nil }
	for name, tc := range map[string]struct {
		floor, other         []string
		verifies             func() (bool, error)
		wantFloor, wantOther int
	}{
		"a verified target":                {floor, other, verified, 0, 0},
		"an unverified floor seat":         {floor, nil, unverified, 1, 0},
		"an unverified non-floor seat":     {nil, other, unverified, 0, 1},
		"both seats on an unverified host": {floor, other, unverified, 1, 1},
	} {
		floorGaps, otherGaps, err := seatGaps(tc.floor, tc.other, tc.verifies)
		if err != nil || len(floorGaps) != tc.wantFloor || len(otherGaps) != tc.wantOther {
			t.Errorf("%s: floor %q other %q (err %v), want %d and %d", name, floorGaps, otherGaps, err, tc.wantFloor, tc.wantOther)
		}
		for _, gap := range append(floorGaps, otherGaps...) {
			if !strings.Contains(gap, launchModelVerificationKey) {
				t.Errorf("%s: gap %q does not name %s", name, gap, launchModelVerificationKey)
			}
		}
	}
	if _, _, err := seatGaps(nil, nil, func() (bool, error) { t.Error("no route asks nothing"); return false, nil }); err != nil {
		t.Error(err)
	}
	if _, _, err := seatGaps(floor, nil, func() (bool, error) { return false, errors.New("unloadable") }); err == nil {
		t.Error("a target that cannot be resolved must fail the guard, never pass it")
	}
}

func TestAgyClaudeRouting_OnlyAFloorAgentsOwnRouteIsAFloorSeat(t *testing.T) {
	for route, want := range map[string]bool{
		"pin auditor agy-claude":                          true,
		"cli_routing agents tdd-engineer agy-claude-tmux": true,
		"pin router agy-claude":                           false,
		"cli_routing agents router agy-claude":            false,
		"cli_routing clis agy-claude":                     false,
		"cli_routing default agy-claude":                  false,
		"cli_routing tiers deep agy-claude":               false,
		"cli_routing work evaluate agy-claude":            false,
	} {
		if got := routesAFloorSeat(route); got != want {
			t.Errorf("routesAFloorSeat(%q) = %v, want %v", route, got, want)
		}
	}
}

func floorChainLeaks(r *cliroute.Router, root string, agents []string) ([]string, error) {
	var leaks []string
	for _, agent := range slices.Sorted(slices.Values(agents)) {
		d, err := r.Resolve(cliroute.Request{Agent: agent, ProjectRoot: root, DefaultModel: unsetDispatchTier})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", agent, err)
		}
		for _, cli := range d.Plan.Candidates {
			if llmroute.Family(cli) != "claude" {
				leaks = append(leaks, fmt.Sprintf("%s reaches %s (chain %v)", agent, cli, d.Plan.Candidates))
			}
		}
	}
	return leaks, nil
}

func TestAgyClaudeRouting_TheFloorStillAdmitsOnlyClaudeCode(t *testing.T) {
	root := checkedInRoot(t)
	r, _, err := buildCLIRouter(root, routingCatalog(root), cliroute.Host{LookPath: everyBinaryPresent})
	if err != nil {
		t.Fatalf("the checked-in table: %v", err)
	}
	leaks, err := floorChainLeaks(r, root, slices.Collect(maps.Keys(profiles.ClaudeFamilyFloor())))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range leaks {
		t.Errorf("floor agent %s: once the floor admits another family (L1c), widen routesAFloorSeat to every policy route that reaches the floor", leak)
	}
}

func TestFloorChainLeaks_NamesEveryNonClaudeCodeCandidate(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "agy-claude", "claude"]}}`)
	r, _, err := buildCLIRouter(root, routingCatalog(root), cliroute.Host{LookPath: everyBinaryPresent})
	if err != nil {
		t.Fatal(err)
	}
	if leaks, err := floorChainLeaks(r, root, []string{"auditor"}); err != nil || len(leaks) != 0 {
		t.Fatalf("auditor (Claude Code only) leaks %q (err %v), want none", leaks, err)
	}
	leaks, err := floorChainLeaks(r, root, []string{"auditor", "router"})
	if err != nil || len(leaks) != 1 || !strings.HasPrefix(leaks[0], "router reaches agy-claude-tmux") {
		t.Fatalf("leaks %q (err %v), want exactly the router's agy-claude entry", leaks, err)
	}
}

func loadPolicyFixture(t *testing.T, body string) policy.Policy {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	pol, err := policy.Load(path)
	if err != nil {
		t.Fatalf("policy.Load(%s): %v", body, err)
	}
	return pol
}

func TestAgyClaudeRouting_TheGuardSeesEachRouteL2CanAdd(t *testing.T) {
	floor := profiles.Profile{CLI: "agy-claude-tmux", AllowedCLIs: []string{"agy-claude", "claude"}, CLIFallback: []string{"claude-tmux"}}
	if got := floorProfileRoutesToAgyClaude("auditor", floor); len(got) != 2 {
		t.Errorf("floor profile routes = %q, want its cli and its allowed_clis entry", got)
	}
	cases := map[string]string{
		"pin":                 `{"pins":{"auditor":{"cli":"agy-claude"},"builder":{"cli":"codex"}}}`,
		"tier":                `{"cli_routing":{"tiers":{"deep":["agy-claude","claude"],"top":["claude"]}}}`,
		"clis":                `{"cli_routing":{"clis":["agy","agy-claude","claude"]}}`,
		"default":             `{"cli_routing":{"default":["agy-claude","claude"]}}`,
		"work role":           `{"cli_routing":{"work":{"audit":["agy-claude","claude"]}}}`,
		"agent rule array":    `{"cli_routing":{"agents":{"auditor":["agy-claude","claude"]}}}`,
		"agent rule object":   `{"cli_routing":{"agents":{"auditor":{"cli":["agy-claude-tmux","claude"],"model":"deep"}}}}`,
		"no agy-claude route": `{"pins":{"builder":{"cli":"agy"}},"cli_routing":{"default":["agy","claude"],"tiers":{"deep":["claude"]}}}`,
	}
	for name, body := range cases {
		routes := policyRoutesToAgyClaude(loadPolicyFixture(t, body))
		if want := name != "no agy-claude route"; (len(routes) == 1) != want || (!want && len(routes) != 0) {
			t.Errorf("%s: routes = %q, want exactly one when the policy names agy-claude, none otherwise", name, routes)
		}
	}
}

func useBridgeManifestFixtures(t *testing.T, manifests map[string]string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "bridge-manifests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range manifests {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

func TestAgyClaudeRouting_OnlyARunnableRuleOnTheResolvedTargetCounts(t *testing.T) {
	const label = `"model_label_regex":"^(?P<footer>\\? for shortcuts)?\\s{2,}(?P<model>\\S.*)$"`
	const base = `{"cli":"agy-tmux","binary":"agy","transport":"tmux",` + label + `}`
	const baseWithRule = `{"cli":"agy-tmux","binary":"agy","transport":"tmux",` + label + `,"model_family":"gemini","launch_model_verification":{"label_wait_s":5}}`
	cases := []struct {
		name      string
		manifests map[string]string
		want      bool
		wantErr   string
	}{
		{"a rule on the target", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude","launch_model_verification":{"label_wait_s":5}}`}, true, ""},
		{"a rule inherited from the base", map[string]string{"agy-tmux": baseWithRule, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude"}`}, true, ""},
		{"a label regex with no rule", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude"}`}, false, ""},
		{"a null that removes the base's rule", map[string]string{"agy-tmux": baseWithRule, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude","launch_model_verification":null}`}, false, ""},
		{"an empty rule", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude","launch_model_verification":{}}`}, false, "label_wait_s"},
		{"a rule with no label to read", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude","model_label_regex":null,"launch_model_verification":{"label_wait_s":5}}`}, false, "model_label_regex"},
		{"a label regex naming no footer", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude","model_label_regex":"^\\s{2,}(?P<model>\\S.*)$","launch_model_verification":{"label_wait_s":5}}`}, false, `names no "footer" group`},
		{"a chained base the loader refuses", map[string]string{"agy-tmux": baseWithRule, "agy-mid-tmux": `{"cli":"agy-mid-tmux","base":"agy-tmux"}`, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-mid-tmux"}`}, false, "itself names a base"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useBridgeManifestFixtures(t, tc.manifests)
			verified, err := gobridge.VerifiesLaunchModel(agyClaudeTarget)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("verified = %v, err = %v; want the loader's refusal naming %q", verified, err, tc.wantErr)
				}
				return
			}
			if err != nil || verified != tc.want {
				t.Fatalf("verified = %v (err %v), want %v", verified, err, tc.want)
			}
		})
	}
}
