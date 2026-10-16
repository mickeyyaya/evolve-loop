package main

import (
	"testing"
)

func TestBuildCycleEnv_PropagatesRequireIntent(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"PATH=/usr/bin",
		"EVOLVE_REQUIRE_INTENT=1",
		"HOME=/Users/x",
	}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_REQUIRE_INTENT"] != "1" {
		t.Fatalf("EVOLVE_REQUIRE_INTENT not propagated; got map=%v", got)
	}
}

func TestBuildCycleEnv_PropagatesSandboxFallback(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_SANDBOX_FALLBACK_ON_EPERM"] != "1" {
		t.Errorf("EVOLVE_SANDBOX_FALLBACK_ON_EPERM not propagated; got=%v", got)
	}
}

func TestBuildCycleEnv_SkipsNonEvolvePrefix(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"PATH=/usr/bin",
		"HOME=/Users/x",
		"ANTHROPIC_API_KEY=sk-secret",
		"EVOLVE_REQUIRE_INTENT=1",
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

func TestBuildCycleEnv_DispatcherFlagsPropagate(t *testing.T) {
	t.Parallel()
	t.Run("resume set", func(t *testing.T) {
		cfg := loopConfig{Strategy: "balanced", Resume: true, Reset: true}
		got := buildCycleEnv(cfg, nil)
		if got["EVOLVE_RESUME"] != "1" {
			t.Errorf("EVOLVE_RESUME not set; got=%v", got)
		}
		if _, present := got["EVOLVE_RESET"]; present {
			t.Errorf("EVOLVE_RESET must not be written to env (dead env write); got=%q", got["EVOLVE_RESET"])
		}
	})
	t.Run("none set", func(t *testing.T) {
		cfg := loopConfig{Strategy: "balanced"}
		got := buildCycleEnv(cfg, nil)
		if _, present := got["EVOLVE_RESUME"]; present {
			t.Errorf("EVOLVE_RESUME must not be set when flag false; got=%q", got["EVOLVE_RESUME"])
		}
	})
}

func TestBuildCycleEnv_MalformedEnvIgnored(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_NO_EQUALS",
		"=EVOLVE_NAMELESS",
		"EVOLVE_REQUIRE_INTENT=1",
	}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_REQUIRE_INTENT"] != "1" {
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

func TestBuildCycleEnv_BroadDocumentedFlagsSurface(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_REQUIRE_INTENT=1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1",
		"EVOLVE_TRIAGE_DISABLE=1",
		"EVOLVE_BUILD_PLANNER=1",
	}
	got := buildCycleEnv(cfg, osEnv)
	for k, want := range map[string]string{
		"EVOLVE_REQUIRE_INTENT":            "1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM": "1",
		"EVOLVE_TRIAGE_DISABLE":            "1",
		"EVOLVE_BUILD_PLANNER":             "1",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}
