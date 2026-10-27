package main

import (
	"testing"
)

func TestCarriedEnv_SkipsNonEvolvePrefix(t *testing.T) {
	t.Parallel()
	osEnv := []string{
		"PATH=/usr/bin",
		"HOME=/Users/x",
		"ANTHROPIC_API_KEY=sk-secret",
		"EVOLVE_PROJECT_ROOT=/repo",
	}
	got := carriedEnv(osEnv)
	for k := range got {
		switch k {
		case "PATH", "HOME", "ANTHROPIC_API_KEY":
			t.Errorf("non-EVOLVE_ var %q leaked into cycle env", k)
		}
	}
}

func TestCarriedEnv_MalformedEnvIgnored(t *testing.T) {
	t.Parallel()
	osEnv := []string{
		"EVOLVE_NO_EQUALS",
		"=EVOLVE_NAMELESS",
		"EVOLVE_PLUGIN_ROOT",
		"EVOLVE_PROJECT_ROOT=/repo",
	}
	got := carriedEnv(osEnv)
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

func TestCarriedEnv_CarriesOnlyTheMarkers(t *testing.T) {
	t.Parallel()
	osEnv := []string{
		"EVOLVE_BYPASS_SHIP_GATE=1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1",
		"EVOLVE_PROJECT_ROOT=/repo",
		"EVOLVE_ROUTING_MODE=static",
		"EVOLVE_SANDBOX=off",
		"EVOLVE_AUDITOR_CLI=claude-tmux",
		"EVOLVE_AUDITOR_MODEL=opus",
		"BRIDGE_ALLOW_BYPASS=1",
		"BRIDGE_RUN_ID=r1",
	}
	got := carriedEnv(osEnv)
	for _, name := range []string{"EVOLVE_BYPASS_SHIP_GATE", "EVOLVE_SANDBOX_FALLBACK_ON_EPERM", "EVOLVE_SANDBOX", "BRIDGE_ALLOW_BYPASS", "BRIDGE_RUN_ID", "EVOLVE_ROUTING_MODE", "EVOLVE_AUDITOR_CLI", "EVOLVE_AUDITOR_MODEL"} {
		if _, present := got[name]; present {
			t.Errorf("%s of the parent reached the cycle env; only the pipeline markers may", name)
		}
	}
	if got["EVOLVE_PROJECT_ROOT"] != "/repo" {
		t.Errorf("EVOLVE_PROJECT_ROOT = %q, want /repo: a marker must still reach the cycle", got["EVOLVE_PROJECT_ROOT"])
	}
}
