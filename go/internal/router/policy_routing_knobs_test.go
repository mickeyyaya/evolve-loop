package router

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestPolicyForProject_ReadsTheRoutingKnobsFromPolicyNotTheEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"router":{"dynamic_routing":"enforce","routing_mode":"static"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_DYNAMIC_ROUTING", "off")
	p := PolicyForProject(root)
	if p.Cfg.Stage != config.StageEnforce || p.Cfg.Mode != config.ModeStaticPreset {
		t.Errorf("stage/mode = %v/%v, want enforce/static from policy.json", p.Cfg.Stage, p.Cfg.Mode)
	}
}

func TestRoutingKnobsOf_ProjectsTheGatesAndRouterKeys(t *testing.T) {
	n := 2
	got := RoutingKnobsOf(policy.GatesConfig{PhaseIO: "off"},
		policy.RouterPolicy{DynamicRouting: "shadow", RoutingMode: "llm", ConditionalMandatory: map[string]string{"tdd": "a==b"}, MaxOptionalInsertions: &n})
	want := config.RoutingKnobs{DynamicRouting: "shadow", RoutingMode: "llm", ConditionalMandatory: map[string]string{"tdd": "a==b"},
		MaxOptionalInsertions: &n, PhaseIO: "off"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RoutingKnobsOf = %+v, want %+v", got, want)
	}
}

func TestApplyPolicy_RunsTheKnobsThenMergesTheMandatoryPhases(t *testing.T) {
	pol := policy.Policy{MandatoryPhases: []string{"retro"}, Router: &policy.RouterPolicy{RoutingMode: "static"}}
	in := config.RoutingConfig{Mandatory: []string{"scout", "ship"}}
	got := ApplyPolicy(config.New(), in, pol)
	if got.Mode != config.ModeStaticPreset || !reflect.DeepEqual(got.Mandatory, []string{"scout", "ship", "retro"}) {
		t.Errorf("ApplyPolicy = mode %v mandatory %v, want static and the merged retro", got.Mode, got.Mandatory)
	}
}

func TestConfigForProject_WarnsOnARefusedPolicyAndUsesTheDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"router":{"routing_mod":"static"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	got, _ := ConfigForProject(root, &warn)
	if got.Mode != config.ModeDynamicLLM || !strings.Contains(warn.String(), "routing_mod") {
		t.Errorf("mode %v warn %q, want the default mode and a WARN that names the key", got.Mode, warn.String())
	}
}
