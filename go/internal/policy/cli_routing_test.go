package policy_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func loadPolicyText(t *testing.T, text string) (policy.Policy, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return policy.Load(path)
}

func TestLoad_CLIRoutingDecodesTheOperatorTable(t *testing.T) {
	p, err := loadPolicyText(t, `{"cli_routing": {
		"clis": ["agy", "claude"],
		"default": ["agy", "claude"],
		"work": {"evaluate": ["claude"]},
		"agents": {"router": {"cli": ["agy"], "model": "balanced"}, "memo": ["agy", "claude"]},
		"tiers": {"deep": ["claude"], "top": ["claude"]},
		"after_chain": "stop"
	}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := &policy.CLIRouting{
		CLIs:    []string{"agy", "claude"},
		Default: []string{"agy", "claude"},
		Work:    map[string][]string{"evaluate": {"claude"}},
		Agents: map[string]policy.AgentRule{
			"router": {CLI: []string{"agy"}, Model: "balanced"},
			"memo":   {CLI: []string{"agy", "claude"}},
		},
		Tiers:      map[string][]string{"deep": {"claude"}, "top": {"claude"}},
		AfterChain: "stop",
	}
	if !reflect.DeepEqual(p.CLIRouting, want) {
		t.Fatalf("CLIRouting = %+v, want %+v", p.CLIRouting, want)
	}
}

func TestLoad_AbsentCLIRoutingIsNil(t *testing.T) {
	p, err := loadPolicyText(t, `{"pins": {}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.CLIRouting != nil {
		t.Fatalf("an absent block must stay nil so the legacy projection applies, got %+v", p.CLIRouting)
	}
}

func TestLoad_CLIRoutingRejectsAnUnknownKey(t *testing.T) {
	_, err := loadPolicyText(t, `{"cli_routing": {"clis": ["claude"], "defualt": ["claude"]}}`)
	if err == nil || !strings.Contains(err.Error(), "defualt") || !strings.Contains(err.Error(), "cli_routing") {
		t.Fatalf("a typo inside the block must fail the load naming the key and the block, got %v", err)
	}
}

func TestLoad_CLIRoutingRejectsAnUnknownKeyInsideAnAgentRule(t *testing.T) {
	_, err := loadPolicyText(t, `{"cli_routing": {"clis": ["claude"], "agents": {"builder": {"cli": ["claude"], "tier": "deep"}}}}`)
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Fatalf("an unknown agent-rule key must fail the load, got %v", err)
	}
}

func TestLoad_KeysOutsideTheCLIRoutingBlockStayLenient(t *testing.T) {
	p, err := loadPolicyText(t, `{"not_a_policy_key": 1, "cli_routing": {"clis": ["claude"]}}`)
	if err != nil {
		t.Fatalf("only the cli_routing block is strict; the rest of policy.json stays lenient: %v", err)
	}
	if p.CLIRouting == nil || !reflect.DeepEqual(p.CLIRouting.CLIs, []string{"claude"}) {
		t.Fatalf("CLIRouting = %+v, want clis [claude]", p.CLIRouting)
	}
}

func TestAgentRule_UnmarshalJSONAcceptsAnArrayOrAnObject(t *testing.T) {
	cases := []struct {
		name, text string
		want       policy.AgentRule
	}{
		{"array", `["agy", "claude"]`, policy.AgentRule{CLI: []string{"agy", "claude"}}},
		{"array with space", ` ["claude"]`, policy.AgentRule{CLI: []string{"claude"}}},
		{"object", `{"cli": ["agy"], "model": "fast"}`, policy.AgentRule{CLI: []string{"agy"}, Model: "fast"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r policy.AgentRule
			if err := r.UnmarshalJSON([]byte(tc.text)); err != nil {
				t.Fatalf("UnmarshalJSON(%s): %v", tc.text, err)
			}
			if !reflect.DeepEqual(r, tc.want) {
				t.Fatalf("got %+v, want %+v", r, tc.want)
			}
		})
	}
}

func TestAgentRule_UnmarshalJSONRefusesAScalarOrAMalformedArray(t *testing.T) {
	for _, text := range []string{`"agy"`, `[1, 2]`, `{"cli": "agy"}`} {
		var r policy.AgentRule
		err := r.UnmarshalJSON([]byte(text))
		if err == nil || !strings.Contains(err.Error(), "agent rule") {
			t.Errorf("UnmarshalJSON(%s) = %v, want an agent-rule error", text, err)
		}
	}
}

func TestCLIRouting_UnmarshalJSONIsStrict(t *testing.T) {
	var c policy.CLIRouting
	if err := c.UnmarshalJSON([]byte(`{"clis": ["claude"], "after_chain": "stop"}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if c.AfterChain != "stop" || !reflect.DeepEqual(c.CLIs, []string{"claude"}) {
		t.Fatalf("decoded %+v", c)
	}
	if err := c.UnmarshalJSON([]byte(`{"clis": ["claude"], "fallback": "stop"}`)); err == nil {
		t.Fatal("an unknown key must be refused")
	}
}

func TestLoad_ANullCLIRoutingBlockOrAgentRuleIsRefused(t *testing.T) {
	cases := map[string]string{
		"block":      `{"cli_routing": null}`,
		"agent rule": `{"cli_routing": {"clis": ["claude"], "agents": {"builder": null}}}`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadPolicyText(t, text); err == nil || !strings.Contains(err.Error(), "null") {
				t.Fatalf("a null must not silently mean the legacy sources or an empty rule, got %v", err)
			}
		})
	}
	var r policy.AgentRule
	if err := r.UnmarshalJSON([]byte(" null ")); err == nil {
		t.Fatal("AgentRule.UnmarshalJSON must refuse null")
	}
}

func TestTierNames_AreTheOneRankOrderedVocabulary(t *testing.T) {
	names := policy.TierNames()
	if !reflect.DeepEqual(names, []string{"fast", "balanced", "deep", "top"}) {
		t.Fatalf("TierNames() = %v", names)
	}
	for rank, name := range names {
		if policy.TierRank(name) != rank+1 || policy.TierName(rank+1) != name {
			t.Errorf("rank %d and name %s disagree", rank+1, name)
		}
	}
	names[0] = "mutated"
	if policy.TierNames()[0] != "fast" {
		t.Fatal("TierNames must return a copy")
	}
	for _, rank := range []int{0, 5, -1} {
		if got := policy.TierName(rank); got != "" {
			t.Errorf("TierName(%d) = %q, want empty", rank, got)
		}
	}
}
