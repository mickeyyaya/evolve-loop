package main

import (
	"testing"
)

func TestBuildCycleEnv_SkipsNonEvolvePrefix(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"PATH=/usr/bin",
		"HOME=/Users/x",
		"ANTHROPIC_API_KEY=sk-secret",
		"EVOLVE_PROJECT_ROOT=/repo",
	}
	got := buildCycleEnv(cfg, osEnv)
	for k := range got {
		switch k {
		case "PATH", "HOME", "ANTHROPIC_API_KEY":
			t.Errorf("non-EVOLVE_ var %q leaked into cycle env", k)
		}
	}
}

func TestBuildCycleEnv_StrategyNotWrittenByDispatcher(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "harden"}
	got := buildCycleEnv(cfg, nil)
	if _, present := got["EVOLVE_STRATEGY"]; present {
		t.Errorf("EVOLVE_STRATEGY must not be written by dispatcher; Strategy flows via Context[\"strategy\"]; got %q", got["EVOLVE_STRATEGY"])
	}
}

func TestBuildCycleEnv_ResumeAndResetWriteNoEnvName(t *testing.T) {
	t.Parallel()
	got := buildCycleEnv(loopConfig{Strategy: "balanced", Resume: true, Reset: true}, nil)
	for _, name := range []string{"EVOLVE_RESUME", "EVOLVE_RESET"} {
		if _, present := got[name]; present {
			t.Errorf("%s is a dead name that no code reads; buildCycleEnv must not write it; got=%v", name, got)
		}
	}
}

func TestBuildCycleEnv_MalformedEnvIgnored(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_NO_EQUALS",
		"=EVOLVE_NAMELESS",
		"EVOLVE_PLUGIN_ROOT",
		"EVOLVE_PROJECT_ROOT=/repo",
	}
	got := buildCycleEnv(cfg, osEnv)
	if _, present := got["EVOLVE_PLUGIN_ROOT"]; present {
		t.Errorf("a marker with no '=' must not reach the cycle env; got=%v", got)
	}
	if got["EVOLVE_PROJECT_ROOT"] != "/repo" {
		t.Errorf("well-formed entry must still parse; got=%v", got)
	}
}

func TestBuildCycleContext_PropagatesGoalText(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{
		Strategy: "ultrathink",
		GoalText: "auto-review pipeline for non-stop autonomy",
	}
	got := buildCycleContext(cfg)
	if got["goal"] != "auto-review pipeline for non-stop autonomy" {
		t.Errorf("goal not propagated; got %q", got["goal"])
	}
	if got["strategy"] != "ultrathink" {
		t.Errorf("strategy not propagated; got %q", got["strategy"])
	}
}

func TestBuildCycleContext_EmptyGoalTextOmitsKey(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	got := buildCycleContext(cfg)
	if _, present := got["goal"]; present {
		t.Errorf("empty GoalText must not add 'goal' key; got %v", got)
	}
}

func TestBuildCycleEnv_CarriesOnlyTheMarkersAndTheControlsNotYetMoved(t *testing.T) {
	t.Parallel()
	osEnv := []string{
		"EVOLVE_BYPASS_SHIP_GATE=1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1",
		"EVOLVE_PROJECT_ROOT=/repo",
		"EVOLVE_ROUTING_MODE=static",
		"EVOLVE_SANDBOX=off",
		"EVOLVE_AUDITOR_CLI=claude-tmux",
		"BRIDGE_ALLOW_BYPASS=1",
		"BRIDGE_RUN_ID=r1",
	}
	got := buildCycleEnv(loopConfig{Strategy: "balanced"}, osEnv)
	for _, name := range []string{"EVOLVE_BYPASS_SHIP_GATE", "EVOLVE_SANDBOX_FALLBACK_ON_EPERM", "BRIDGE_ALLOW_BYPASS", "BRIDGE_RUN_ID"} {
		if _, present := got[name]; present {
			t.Errorf("%s of the parent reached the cycle env; only the pipeline markers and the controls not yet moved may", name)
		}
	}
	for name, want := range map[string]string{"EVOLVE_PROJECT_ROOT": "/repo", "EVOLVE_ROUTING_MODE": "static", "EVOLVE_SANDBOX": "off", "EVOLVE_AUDITOR_CLI": "claude-tmux"} {
		if got[name] != want {
			t.Errorf("%s = %q, want %q: a marker or a control not yet moved must still reach the cycle", name, got[name], want)
		}
	}
}
