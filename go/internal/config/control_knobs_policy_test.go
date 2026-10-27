package config

import (
	"io/fs"
	"reflect"
	"testing"
)

func TestLoad_IgnoresTheMovedRoutingEnvironmentDials(t *testing.T) {
	env := map[string]string{
		"EVOLVE_DYNAMIC_ROUTING": "enforce", "EVOLVE_ROUTING_MODE": "static", "EVOLVE_COMMIT_EVIDENCE": "enforce",
		"EVOLVE_PHASE_IO": "off", "EVOLVE_MANDATORY_PHASES": "scout,build", "EVOLVE_CONDITIONAL_MANDATORY": "tdd:build_size>=9",
		"EVOLVE_MAX_OPTIONAL_INSERTIONS": "9",
	}
	got, ws := New(WithReadFile(func(string) ([]byte, error) { return nil, fs.ErrNotExist })).Load("absent.json", env)
	if !reflect.DeepEqual(got, defaults()) {
		t.Errorf("Load with the moved dials set = %+v, want the compiled defaults: the routing reads policy.json, not the environment", got)
	}
	if len(ws) != 0 {
		t.Errorf("warnings = %+v, want none", ws)
	}
}

func TestApplyRoutingKnobs_ProjectsTheMovedRoutingKnobs(t *testing.T) {
	l, _ := observed(t)
	zero := 0
	k := RoutingKnobs{DynamicRouting: "enforce", RoutingMode: "static", ConditionalMandatory: map[string]string{"tdd": "build_size>=3"},
		MaxOptionalInsertions: &zero, PhaseIO: "advisory"}
	got, ws := l.ApplyRoutingKnobs(defaults(), k)
	if got.Stage != StageEnforce || got.Mode != ModeStaticPreset || got.MaxInsertions != 0 {
		t.Errorf("stage/mode/insertions = %v/%v/%d, want enforce/static/0", got.Stage, got.Mode, got.MaxInsertions)
	}
	if got.PhaseIO != StageAdvisory {
		t.Errorf("phase io = %v, want advisory", got.PhaseIO)
	}
	if want := mustCondRule("build_size>=3"); !reflect.DeepEqual(got.Conditional["tdd"], want) {
		t.Errorf("conditional tdd = %+v, want %+v", got.Conditional["tdd"], want)
	}
	if len(ws) != 0 {
		t.Errorf("warnings = %+v, want none", ws)
	}
}

func TestApplyRoutingKnobs_AbsentKnobsKeepTheRegistryValues(t *testing.T) {
	l, _ := observed(t)
	in := defaults()
	in.Stage, in.Mode, in.MaxInsertions, in.PhaseIO = StageShadow, ModeStaticPreset, 7, StageShadow
	got, _ := l.ApplyRoutingKnobs(in, RoutingKnobs{})
	if got.Stage != StageShadow || got.Mode != ModeStaticPreset || got.MaxInsertions != 7 || got.PhaseIO != StageShadow {
		t.Errorf("absent knobs changed the registry values: %+v", got.RolloutStages)
	}
	if !reflect.DeepEqual(got.Conditional, in.Conditional) {
		t.Errorf("conditional = %+v, want %+v", got.Conditional, in.Conditional)
	}
}

func TestApplyRoutingKnobs_WarnsOnAMovedKnobTypoWithItsPolicyKey(t *testing.T) {
	l, _ := observed(t)
	_, ws := l.ApplyRoutingKnobs(defaults(), RoutingKnobs{PhaseIO: "enfroce", RoutingMode: "bogus"})
	var keys []string
	for _, w := range ws {
		keys = append(keys, w.Fields["key"])
	}
	if want := []string{"routing_mode", "gates.phase_io"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("warning keys = %v, want %v", keys, want)
	}
}

func withKnobs(cfg RoutingConfig, edit func(*RoutingKnobs)) (RoutingConfig, []Warning) {
	var k RoutingKnobs
	edit(&k)
	return New().ApplyRoutingKnobs(cfg, k)
}

func TestLoader_ApplyRoutingKnobs_StampsAndEmitsThePolicyWarnings(t *testing.T) {
	l, events := observed(t)
	got, ws := l.ApplyRoutingKnobs(defaults(), RoutingKnobs{RoutingMode: "static", PhaseIO: "bogus"})
	if got.Mode != ModeStaticPreset || got.PhaseIO != StageOff {
		t.Errorf("mode/phase io = %v/%v, want static/off", got.Mode, got.PhaseIO)
	}
	if len(ws) != 1 || ws[0].Fields["step"] != "policy" || ws[0].Fields["key"] != "gates.phase_io" || len(*events) != 1 {
		t.Errorf("warnings %+v events %d, want one stamped gates.phase_io warning emitted once", ws, len(*events))
	}
}

func TestApplyRoutingKnobs_AConfigWithNoConditionalMapGetsOne(t *testing.T) {
	got, ws := New().ApplyRoutingKnobs(RoutingConfig{}, RoutingKnobs{ConditionalMandatory: map[string]string{"tdd": "build_size>=3"}})
	if len(ws) != 0 || !reflect.DeepEqual(got.Conditional["tdd"], mustCondRule("build_size>=3")) {
		t.Errorf("conditional = %+v warnings = %+v, want the tdd rule", got.Conditional, ws)
	}
}
