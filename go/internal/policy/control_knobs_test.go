package policy_test

import (
	"maps"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestParse_ControlKnobKeysDecodeStrictly(t *testing.T) {
	pol, err := policy.Parse([]byte(`{
		"router": {"dynamic_routing": "enforce", "routing_mode": "static", "conditional_mandatory": {"tdd": "build_size >= 3"}, "max_optional_insertions": 0},
		"gates": {"phase_io": "shadow"},
		"cli_health": {"enabled": false},
		"bridge": {"max_concurrent": {"claude": 2}},
		"workflow": {"intent_delta": true}
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rt := pol.RouterConfig()
	if rt.DynamicRouting != "enforce" || rt.RoutingMode != "static" || rt.ConditionalMandatory["tdd"] != "build_size >= 3" {
		t.Errorf("RouterConfig() = %+v, want the router control keys", rt)
	}
	if rt.MaxOptionalInsertions == nil || *rt.MaxOptionalInsertions != 0 {
		t.Errorf("RouterConfig().MaxOptionalInsertions = %v, want an explicit 0", rt.MaxOptionalInsertions)
	}
	if g := pol.GatesConfig(); g.PhaseIO != "shadow" {
		t.Errorf("GatesConfig() = %+v, want phase_io shadow", g)
	}
	if pol.CLIHealthConfig().On() {
		t.Error("CLIHealthConfig().On() = true, want false for cli_health.enabled false")
	}
	if got := pol.BridgeConfig().MaxConcurrent; !maps.Equal(got, map[string]int{"claude": 2}) {
		t.Errorf("BridgeConfig().MaxConcurrent = %v, want claude 2", got)
	}
	if !pol.WorkflowConfig().IntentDelta {
		t.Error("WorkflowConfig().IntentDelta = false, want true")
	}
}

func TestControlKnobDefaults_KeepTheCompiledBehavior(t *testing.T) {
	var pol policy.Policy
	rt := pol.RouterConfig()
	if rt.DynamicRouting != "" || rt.RoutingMode != "" || rt.ConditionalMandatory != nil || rt.MaxOptionalInsertions != nil {
		t.Errorf("RouterConfig() = %+v, want no routing override without a key", rt)
	}
	if g := pol.GatesConfig(); g.PhaseIO != "" {
		t.Errorf("GatesConfig() = %+v, want no stage override without a key", g)
	}
	if !pol.CLIHealthConfig().On() {
		t.Error("CLIHealthConfig().On() = false, want true without a key")
	}
	on := true
	if !(policy.CLIHealthConfig{Enabled: &on}).On() {
		t.Error("CLIHealthConfig{Enabled: true}.On() = false, want true")
	}
	if pol.WorkflowConfig().IntentDelta {
		t.Error("WorkflowConfig().IntentDelta = true, want false without a key")
	}
}

func TestParse_RefusesAnUnknownControlKnobKey(t *testing.T) {
	for _, raw := range []string{`{"router": {"routing_mod": "static"}}`, `{"gates": {"commit_evidence": "enforce"}}`} {
		if _, err := policy.Parse([]byte(raw)); err == nil {
			t.Errorf("Parse accepted %s, want the strict decoder to refuse it", raw)
		}
	}
}
