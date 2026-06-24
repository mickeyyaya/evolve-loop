package main

import (
	"testing"
)

// TestBuildCycleEnv_PropagatesRequireIntent is the regression test for
// the cycle-108 silent-skip bug: EVOLVE_REQUIRE_INTENT=1 in the
// operator shell MUST land in CycleRequest.Env so the orchestrator's
// intent gate at orchestrator.go:126 evaluates true.
func TestBuildCycleEnv_PropagatesRequireIntent(t *testing.T) {
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

// TestBuildCycleEnv_PropagatesSandboxFallback covers the second
// documented STRICT-MODE flag that was also silently dropped.
func TestBuildCycleEnv_PropagatesSandboxFallback(t *testing.T) {
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_SANDBOX_FALLBACK_ON_EPERM"] != "1" {
		t.Errorf("EVOLVE_SANDBOX_FALLBACK_ON_EPERM not propagated; got=%v", got)
	}
}

// TestBuildCycleEnv_SkipsNonEvolvePrefix ensures only the documented
// operator surface (EVOLVE_*) makes it through — other env vars stay
// out of the cycle env to avoid surprising subagents.
func TestBuildCycleEnv_SkipsNonEvolvePrefix(t *testing.T) {
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

// TestBuildCycleEnv_StrategyNotWrittenByDispatcher asserts that
// EVOLVE_STRATEGY is NOT written to the env map by the dispatcher.
// Strategy flows via Context["strategy"] — not via the cycle env.
func TestBuildCycleEnv_StrategyNotWrittenByDispatcher(t *testing.T) {
	cfg := loopConfig{Strategy: "harden"}
	got := buildCycleEnv(cfg, nil)
	if _, present := got["EVOLVE_STRATEGY"]; present {
		t.Errorf("EVOLVE_STRATEGY must not be written by dispatcher; Strategy flows via Context[\"strategy\"]; got %q", got["EVOLVE_STRATEGY"])
	}
}

// TestBuildCycleEnv_DispatcherFlagsPropagate covers the explicitly-set
// dispatcher IPC flags (Resume) — present iff bool set.
// ConsensusAudit is no longer written to the cycle env; it is configured
// via policy.json workflow.consensus_audit_enabled instead.
// EVOLVE_RESET is no longer written: cfg.Reset is consumed at cmd_loop.go
// before buildCycleEnv is called (dead env write removed, cycle-44).
func TestBuildCycleEnv_DispatcherFlagsPropagate(t *testing.T) {
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

// TestBuildCycleEnv_MalformedEnvIgnored guards against panics on env
// lines without '=' (rare but possible from low-level callers).
func TestBuildCycleEnv_MalformedEnvIgnored(t *testing.T) {
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_NO_EQUALS", // no '='
		"=EVOLVE_NAMELESS", // empty name
		"EVOLVE_REQUIRE_INTENT=1",
	}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_REQUIRE_INTENT"] != "1" {
		t.Errorf("well-formed entry must still parse; got=%v", got)
	}
}

// TestBuildCycleContext_PropagatesGoalText is the regression test for
// the cycle-108 silent-drop bug #3: --goal-text "..." flag value must
// land in CycleRequest.Context["goal"] so the Intent persona can
// structure intent.md around the operator's actual goal rather than
// inferring from leftover workspace artifacts.
func TestBuildCycleContext_PropagatesGoalText(t *testing.T) {
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
	// When --goal-text was not passed (operator used --goal-hash or
	// resume), the "goal" key must NOT appear in Context — phases
	// check for empty and skip the prompt-line, but defensive: keep
	// the map minimal.
	cfg := loopConfig{Strategy: "balanced"} // no GoalText
	got := buildCycleContext(cfg)
	if _, present := got["goal"]; present {
		t.Errorf("empty GoalText must not add 'goal' key; got %v", got)
	}
}

// TestBuildCycleEnv_BroadDocumentedFlagsSurface spot-checks several
// documented EVOLVE_* flags from CLAUDE.md (intent, sandbox, triage,
// build-planner) to lock in the propagation contract.
func TestBuildCycleEnv_BroadDocumentedFlagsSurface(t *testing.T) {
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
