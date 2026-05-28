package runner

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Workstream G1 — resolver tests for the per-phase CLI chain.
//
// Resolution order for the PRIMARY: EVOLVE_<AGENT>_CLI > EVOLVE_CLI >
// profile.cli > "claude-tmux". The fallback chain prepends primary, then
// dedup's profile.cli_fallback. Triggers come from
// profile.cli_fallback_on_exit, defaulting to {80, 127}.

func TestResolveCLIChain_EnvPerAgentBeatsProfile(t *testing.T) {
	// Per-agent env override is the highest-precedence source.
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "claude-tmux"}
	got := resolveCLIChain("auditor", env, prof)
	if got.candidates[0] != "claude-tmux" {
		t.Errorf("primary=%q, want claude-tmux (per-agent env wins)", got.candidates[0])
	}
	if got.primarySource != "env(EVOLVE_AUDITOR_CLI)" {
		t.Errorf("source=%q, want env(EVOLVE_AUDITOR_CLI)", got.primarySource)
	}
}

func TestResolveCLIChain_GlobalEnvBeatsProfile(t *testing.T) {
	// EVOLVE_CLI is the legacy global override — beats profile when no
	// per-agent env is set.
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{"EVOLVE_CLI": "agy-tmux"}
	got := resolveCLIChain("auditor", env, prof)
	if got.candidates[0] != "agy-tmux" {
		t.Errorf("primary=%q, want agy-tmux", got.candidates[0])
	}
	if got.primarySource != "env(EVOLVE_CLI)" {
		t.Errorf("source=%q, want env(EVOLVE_CLI)", got.primarySource)
	}
}

func TestResolveCLIChain_PerAgentBeatsGlobalEnv(t *testing.T) {
	// When BOTH per-agent and global env are set, per-agent wins.
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{
		"EVOLVE_AUDITOR_CLI": "claude-tmux",
		"EVOLVE_CLI":         "agy-tmux",
	}
	got := resolveCLIChain("auditor", env, prof)
	if got.candidates[0] != "claude-tmux" {
		t.Errorf("primary=%q, want claude-tmux (per-agent beats global)", got.candidates[0])
	}
}

func TestResolveCLIChain_ProfileUsedWhenNoEnv(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	got := resolveCLIChain("auditor", nil, prof)
	if got.candidates[0] != "codex-tmux" {
		t.Errorf("primary=%q, want codex-tmux (from profile)", got.candidates[0])
	}
	if got.primarySource != "profile.auditor.cli" {
		t.Errorf("source=%q, want profile.auditor.cli", got.primarySource)
	}
}

func TestResolveCLIChain_DefaultWhenAllEmpty(t *testing.T) {
	got := resolveCLIChain("auditor", nil, nil)
	if got.candidates[0] != "claude-tmux" {
		t.Errorf("primary=%q, want claude-tmux (final default)", got.candidates[0])
	}
	if got.primarySource != "default" {
		t.Errorf("source=%q, want default", got.primarySource)
	}
}

func TestResolveCLIChain_FallbackDedup(t *testing.T) {
	// If the primary appears in profile.cli_fallback, it must be dropped
	// to avoid retrying the same CLI on the same failure.
	prof := &profiles.Profile{
		CLI:         "codex-tmux",
		CLIFallback: []string{"codex-tmux", "claude-tmux", "agy-tmux"},
	}
	got := resolveCLIChain("auditor", nil, prof)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("candidates=%v, want %v (primary dedup'd from fallback)", got.candidates, want)
	}
}

func TestResolveCLIChain_FallbackDedupPreservesOrder(t *testing.T) {
	// Internal dup in the fallback list also dropped — first occurrence
	// wins so operator order is preserved.
	prof := &profiles.Profile{
		CLI:         "codex-tmux",
		CLIFallback: []string{"claude-tmux", "agy-tmux", "claude-tmux"},
	}
	got := resolveCLIChain("auditor", nil, prof)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("candidates=%v, want %v", got.candidates, want)
	}
}

func TestResolveCLIChain_DefaultTriggers(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	got := resolveCLIChain("auditor", nil, prof)
	want := []int{80, 127}
	if !reflect.DeepEqual(got.triggers, want) {
		t.Errorf("triggers=%v, want %v (default = REPL boot + missing binary)", got.triggers, want)
	}
}

func TestResolveCLIChain_CustomTriggers(t *testing.T) {
	// Operator can extend the trigger set per-agent (e.g., to include 81
	// ExitArtifactTimeout for a more aggressive policy).
	prof := &profiles.Profile{
		CLI:               "codex-tmux",
		CLIFallbackOnExit: []int{80, 127, 81, 2},
	}
	got := resolveCLIChain("auditor", nil, prof)
	if !reflect.DeepEqual(got.triggers, []int{80, 127, 81, 2}) {
		t.Errorf("triggers=%v, want operator-extended list", got.triggers)
	}
}

func TestCLIChain_TriggersFallback(t *testing.T) {
	c := cliChain{triggers: []int{80, 127}}
	cases := []struct {
		exit int
		want bool
	}{
		{0, false},  // success
		{80, true},  // REPL boot timeout → trigger
		{81, false}, // artifact timeout NOT a default trigger
		{127, true}, // missing binary → trigger
		{2, false},  // safety gate — not a trigger
		{1, false},  // generic failure → don't retry
	}
	for _, tc := range cases {
		if got := c.triggersFallback(tc.exit); got != tc.want {
			t.Errorf("triggersFallback(%d)=%v, want %v", tc.exit, got, tc.want)
		}
	}
}

func TestResolveCLIChain_TrimsWhitespace(t *testing.T) {
	// JSON-loaded fields commonly carry trailing whitespace from copy-paste.
	// The chain must tolerate it without producing phantom "  " candidates.
	prof := &profiles.Profile{
		CLI:         "codex-tmux",
		CLIFallback: []string{"  claude-tmux\n", "\tagy-tmux ", "  "},
	}
	got := resolveCLIChain("auditor", nil, prof)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("candidates=%v, want %v (whitespace stripped, empty rejected)", got.candidates, want)
	}
}
