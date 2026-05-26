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

// TestBuildCycleEnv_CLIOverridesEnv asserts that dispatcher-derived
// choices win over operator env. If the operator sets
// EVOLVE_STRATEGY=balanced but runs `evolve loop ... harden`, the CLI
// arg should produce Strategy=harden.
func TestBuildCycleEnv_CLIOverridesEnv(t *testing.T) {
	cfg := loopConfig{Strategy: "harden"}
	osEnv := []string{"EVOLVE_STRATEGY=balanced"}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_STRATEGY"] != "harden" {
		t.Errorf("CLI must win over env; got %q want harden", got["EVOLVE_STRATEGY"])
	}
}

// TestBuildCycleEnv_DispatcherFlagsPropagate covers the 3 explicitly-set
// dispatcher flags (ConsensusAudit, Resume, Reset) — present iff bool set.
func TestBuildCycleEnv_DispatcherFlagsPropagate(t *testing.T) {
	t.Run("all set", func(t *testing.T) {
		cfg := loopConfig{Strategy: "balanced", ConsensusAudit: true, Resume: true, Reset: true}
		got := buildCycleEnv(cfg, nil)
		for _, k := range []string{"EVOLVE_CONSENSUS_AUDIT", "EVOLVE_RESUME", "EVOLVE_RESET"} {
			if got[k] != "1" {
				t.Errorf("%s not set; got=%v", k, got)
			}
		}
	})
	t.Run("none set", func(t *testing.T) {
		cfg := loopConfig{Strategy: "balanced"}
		got := buildCycleEnv(cfg, nil)
		for _, k := range []string{"EVOLVE_CONSENSUS_AUDIT", "EVOLVE_RESUME", "EVOLVE_RESET"} {
			if _, present := got[k]; present {
				t.Errorf("%s must not be set when flag false; got=%q", k, got[k])
			}
		}
	})
}

// TestBuildCycleEnv_MalformedEnvIgnored guards against panics on env
// lines without '=' (rare but possible from low-level callers).
func TestBuildCycleEnv_MalformedEnvIgnored(t *testing.T) {
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_NO_EQUALS",       // no '='
		"=EVOLVE_NAMELESS",       // empty name
		"EVOLVE_REQUIRE_INTENT=1",
	}
	got := buildCycleEnv(cfg, osEnv)
	if got["EVOLVE_REQUIRE_INTENT"] != "1" {
		t.Errorf("well-formed entry must still parse; got=%v", got)
	}
}

// TestBuildCycleEnv_BroadDocumentedFlagsSurface spot-checks several
// documented EVOLVE_* flags from CLAUDE.md (intent, sandbox, triage,
// build-planner, stdout-filter) to lock in the propagation contract.
func TestBuildCycleEnv_BroadDocumentedFlagsSurface(t *testing.T) {
	cfg := loopConfig{Strategy: "balanced"}
	osEnv := []string{
		"EVOLVE_REQUIRE_INTENT=1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1",
		"EVOLVE_TRIAGE_DISABLE=1",
		"EVOLVE_BUILD_PLANNER=1",
		"EVOLVE_STDOUT_FILTER=off",
		"EVOLVE_STRICT_AUDIT=1",
		"EVOLVE_BATCH_BUDGET_CAP=60",
	}
	got := buildCycleEnv(cfg, osEnv)
	for k, want := range map[string]string{
		"EVOLVE_REQUIRE_INTENT":            "1",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM": "1",
		"EVOLVE_TRIAGE_DISABLE":            "1",
		"EVOLVE_BUILD_PLANNER":             "1",
		"EVOLVE_STDOUT_FILTER":             "off",
		"EVOLVE_STRICT_AUDIT":              "1",
		"EVOLVE_BATCH_BUDGET_CAP":          "60",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}
