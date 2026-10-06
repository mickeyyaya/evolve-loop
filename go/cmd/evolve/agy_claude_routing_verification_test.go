package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const (
	agyClaudeFamily               = "agy-claude"
	agyClaudeTarget               = "agy-claude-tmux"
	launchModelVerificationKey    = "launch_model_verification"
	agyClaudeVerificationPlanLink = "docs/plans/cli-routing-table-2026-10.md (L1c prerequisite 1) and C5 of docs/plans/model-currency-2026-10.md"
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

func launchModelVerificationDeclared(target string) (bool, error) {
	object, err := gobridge.ManifestObject(target)
	if err != nil {
		return false, err
	}
	return declaresARule(object[launchModelVerificationKey]), nil
}

func declaresARule(value any) bool {
	rule, isObject := value.(map[string]any)
	return value != nil && (!isObject || len(rule) > 0)
}

func realTreeAgyClaudeRoutes(t *testing.T) []string {
	t.Helper()
	root := checkedInRoot(t)
	loader := profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles"))
	var routes []string
	for name := range profiles.ClaudeFamilyFloor() {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("floor profile %s: %v", name, err)
		}
		routes = append(routes, floorProfileRoutesToAgyClaude(name, p)...)
	}
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load policy.json: %v", err)
	}
	return append(routes, policyRoutesToAgyClaude(pol)...)
}

func TestAgyClaudeRouting_WaitsForALaunchTimeModelVerificationRule(t *testing.T) {
	routes := realTreeAgyClaudeRoutes(t)
	if len(routes) == 0 {
		return
	}
	declared, err := launchModelVerificationDeclared(agyClaudeTarget)
	if err != nil {
		t.Fatalf("resolve %s: %v", agyClaudeTarget, err)
	}
	if !declared {
		slices.Sort(routes)
		t.Errorf("routing reaches %s through %q, but %s declares no %q rule: agy boots its default Gemini model for a --model it does not recognize, so a Claude seat would silently run Gemini. Land the rule first: %s",
			agyClaudeFamily, routes, agyClaudeTarget, launchModelVerificationKey, agyClaudeVerificationPlanLink)
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

func TestAgyClaudeRouting_OnlyANonEmptyRuleOnTheResolvedTargetCounts(t *testing.T) {
	const base = `{"cli":"agy-tmux","binary":"agy","transport":"tmux"}`
	const baseWithRule = `{"cli":"agy-tmux","binary":"agy","transport":"tmux","launch_model_verification":{"label":"model_label_regex"}}`
	cases := []struct {
		name      string
		manifests map[string]string
		want      bool
		wantErr   string
	}{
		{"a rule on the target", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","launch_model_verification":{"label":"model_label_regex"}}`}, true, ""},
		{"a rule inherited from the base", map[string]string{"agy-tmux": baseWithRule, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux"}`}, true, ""},
		{"an empty rule", map[string]string{"agy-tmux": base, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","launch_model_verification":{}}`}, false, ""},
		{"a null that removes the base's rule", map[string]string{"agy-tmux": baseWithRule, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-tmux","launch_model_verification":null}`}, false, ""},
		{"a chained base the loader refuses", map[string]string{"agy-tmux": baseWithRule, "agy-mid-tmux": `{"cli":"agy-mid-tmux","base":"agy-tmux"}`, "agy-claude-tmux": `{"cli":"agy-claude-tmux","base":"agy-mid-tmux"}`}, false, "itself names a base"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useBridgeManifestFixtures(t, tc.manifests)
			declared, err := launchModelVerificationDeclared(agyClaudeTarget)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("declared = %v, err = %v; want the loader's refusal naming %q", declared, err, tc.wantErr)
				}
				return
			}
			if err != nil || declared != tc.want {
				t.Fatalf("declared = %v (err %v), want %v", declared, err, tc.want)
			}
		})
	}
}
