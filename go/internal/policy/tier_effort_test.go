package policy_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCLIRoutingTiers_TakeTheListAndTheObjectForm(t *testing.T) {
	p, err := loadPolicyText(t, `{"cli_routing": {"clis": ["claude"], "tiers": {
		"deep": ["agy-claude", "claude"],
		"top": {"clis": ["claude"], "effort": "high"},
		"fast": {"effort": "low"}}}}`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := map[string]policy.TierRule{
		"deep": {CLIs: []string{"agy-claude", "claude"}},
		"top":  {CLIs: []string{"claude"}, Effort: "high"},
		"fast": {Effort: "low"},
	}
	if got := p.CLIRouting.Tiers; !reflect.DeepEqual(got, want) {
		t.Fatalf("tiers = %+v, want %+v", got, want)
	}
}

func TestCLIRoutingTiers_RefuseAnUnknownFieldAndNull(t *testing.T) {
	for _, body := range []string{
		`{"cli_routing": {"clis": ["claude"], "tiers": {"deep": {"clis": ["claude"], "efort": "high"}}}}`,
		`{"cli_routing": {"clis": ["claude"], "tiers": {"deep": null}}}`,
	} {
		_, err := loadPolicyText(t, body)
		if err == nil || !strings.Contains(err.Error(), "tier rule") {
			t.Errorf("load(%s) err = %v, want a tier rule error", body, err)
		}
	}
}

func TestTierRule_MarshalsAChainOnlyRuleAsAList(t *testing.T) {
	cases := []struct {
		rule policy.TierRule
		want string
	}{
		{policy.TierRule{CLIs: []string{"claude"}}, `["claude"]`},
		{policy.TierRule{CLIs: []string{"claude"}, Effort: "high"}, `{"clis":["claude"],"effort":"high"}`},
		{policy.TierRule{Effort: "low"}, `{"effort":"low"}`},
	}
	for _, tc := range cases {
		raw, err := tc.rule.MarshalJSON()
		if err != nil || string(raw) != tc.want {
			t.Errorf("MarshalJSON(%+v) = %s, %v; want %s", tc.rule, raw, err, tc.want)
		}
		if nested, err := json.Marshal(map[string]policy.TierRule{"t": tc.rule}); err != nil || string(nested) != `{"t":`+tc.want+`}` {
			t.Errorf("json.Marshal in a map = %s, %v; want the same form", nested, err)
		}
	}
}

func TestAgentRule_TakesAnEffortBesideTheModel(t *testing.T) {
	p, err := loadPolicyText(t, `{"cli_routing": {"clis": ["claude"], "agents": {
		"auditor": {"cli": ["claude"], "model": "deep", "effort": "high"},
		"scout": {"effort": "low"}}}}`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := p.CLIRouting.Agents["auditor"]; got.Effort != "high" || got.Model != "deep" {
		t.Errorf("agents.auditor = %+v, want model deep and effort high", got)
	}
	if got := p.CLIRouting.Agents["scout"]; got.Effort != "low" || len(got.CLI) != 0 {
		t.Errorf("agents.scout = %+v, want only the effort", got)
	}
}

func TestEffortTable_ResolvesAgentThenTierThenTheCompiledDefault(t *testing.T) {
	block := &policy.CLIRouting{
		Tiers:  map[string]policy.TierRule{"deep": {Effort: "high"}},
		Agents: map[string]policy.AgentRule{"scout": {Effort: "low"}, "audit": {CLI: []string{"claude"}}},
	}
	table := policy.Policy{CLIRouting: block}.Efforts()
	cases := []struct {
		name, tier   string
		agents       []string
		want, source string
	}{
		{"agent-wins", "deep", []string{"scout"}, "low", "cli_routing.agents.scout"},
		{"agent-by-its-second-key", "deep", []string{"auditor", "scout"}, "low", "cli_routing.agents.scout"},
		{"tier-when-the-agent-has-none", "deep", []string{"audit"}, "high", "cli_routing.tiers.deep"},
		{"default-fast", "fast", nil, "low", "default"},
		{"default-balanced", "balanced", nil, "medium", "default"},
		{"default-deep", "deep", []string{"nobody"}, "high", "cli_routing.tiers.deep"},
		{"default-top", "top", nil, "medium", "default"},
		{"no-tier-no-agent", "", nil, "", ""},
		{"no-tier-but-an-agent", "", []string{"scout"}, "low", "cli_routing.agents.scout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, source := table.Resolve(tc.tier, tc.agents...)
			if got != tc.want || source != tc.source {
				t.Errorf("Resolve(%q, %v) = %q from %q, want %q from %q", tc.tier, tc.agents, got, source, tc.want, tc.source)
			}
		})
	}
}

func TestEffortTable_ZeroValueResolvesTheCompiledDefaults(t *testing.T) {
	want := map[string]string{"fast": "low", "balanced": "medium", "deep": "medium", "top": "medium"}
	for tier, effort := range want {
		if got, source := (policy.EffortTable{}).Resolve(tier); got != effort || source != "default" {
			t.Errorf("zero table Resolve(%s) = %q from %q, want %q from default", tier, got, source, effort)
		}
	}
}

func TestEffortLevels_AreTheOrderedVocabularyAndACopy(t *testing.T) {
	want := []string{"low", "medium", "high", "xhigh", "max"}
	got := policy.EffortLevels()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffortLevels() = %v, want %v", got, want)
	}
	got[0] = "edited"
	if policy.EffortLevels()[0] != "low" {
		t.Fatal("a caller edit leaked into the effort vocabulary")
	}
}

func TestLoad_RefusesAnUnknownEffortLevelWrittenByHand(t *testing.T) {
	for _, body := range []string{
		`{"cli_routing": {"clis": ["claude"], "tiers": {"deep": {"effort": "hihg"}}}}`,
		`{"cli_routing": {"clis": ["claude"], "agents": {"scout": {"effort": "ultra"}}}}`,
	} {
		_, err := loadPolicyText(t, body)
		if err == nil || !strings.Contains(err.Error(), "(levels: low, medium, high, xhigh, max)") {
			t.Errorf("load(%s) err = %v, want the unknown-effort refusal of the compiler", body, err)
		}
	}
}

func TestValidateEffort_AcceptsTheLevelsAndRefusesTheRest(t *testing.T) {
	for _, level := range policy.EffortLevels() {
		if err := policy.ValidateEffort(level); err != nil {
			t.Errorf("ValidateEffort(%q) = %v, want nil", level, err)
		}
	}
	if err := policy.ValidateEffort("ultra"); err == nil || err.Error() != `unknown effort "ultra" (levels: low, medium, high, xhigh, max)` {
		t.Errorf("ValidateEffort(ultra) = %v, want the named refusal", err)
	}
}

func TestTierRule_AnEmptyRuleDoesNotMarshal(t *testing.T) {
	if raw, err := (policy.TierRule{}).MarshalJSON(); err == nil {
		t.Fatalf("MarshalJSON(empty) = %s, want an error: an empty rule names no CLI and no effort", raw)
	}
}
