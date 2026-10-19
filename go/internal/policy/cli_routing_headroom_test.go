package policy_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAgentRule_DecodesTheHeadroomOrderAndItsLimit(t *testing.T) {
	p, err := policy.Load(filepath.Join("testdata", "headroom-builder.policy.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := policy.AgentRule{CLI: []string{"agy", "agy-claude", "claude"}, Model: "balanced", Order: "headroom", HeadroomPct: 60}
	if got := p.CLIRouting.Agents["builder"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("agents.builder = %+v, want %+v", got, want)
	}
}

func TestAgentRule_ARuleWithoutTheOrderKeysDecodesAsBefore(t *testing.T) {
	p, err := loadPolicyText(t, `{"cli_routing": {"clis": ["claude"], "agents": {"builder": {"cli": ["claude"], "model": "balanced"}}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := p.CLIRouting.Agents["builder"]; got.Order != "" || got.HeadroomPct != 0 {
		t.Fatalf("a rule without order keys must leave them zero, got %+v", got)
	}
}
