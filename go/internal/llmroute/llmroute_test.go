package llmroute

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// --- CLI chain (ported from runner/cli_chain_test.go; same matrix, now via Resolve) ---

func TestResolve_EnvPerAgentBeatsProfile(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "claude-tmux"}
	got := Resolve("auditor", "audit", "auto", env, prof, nil)
	if got.Candidates[0] != "claude-tmux" {
		t.Errorf("primary=%q, want claude-tmux (per-agent env wins)", got.Candidates[0])
	}
	if got.PrimarySource != "env(EVOLVE_AUDITOR_CLI)" {
		t.Errorf("source=%q, want env(EVOLVE_AUDITOR_CLI)", got.PrimarySource)
	}
}

func TestResolve_GlobalEnvBeatsProfile(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{"EVOLVE_CLI": "agy-tmux"}
	got := Resolve("auditor", "audit", "auto", env, prof, nil)
	if got.Candidates[0] != "agy-tmux" || got.PrimarySource != "env(EVOLVE_CLI)" {
		t.Errorf("got %q/%q, want agy-tmux/env(EVOLVE_CLI)", got.Candidates[0], got.PrimarySource)
	}
}

func TestResolve_PerAgentBeatsGlobalEnv(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "claude-tmux", "EVOLVE_CLI": "agy-tmux"}
	got := Resolve("auditor", "audit", "auto", env, prof, nil)
	if got.Candidates[0] != "claude-tmux" {
		t.Errorf("primary=%q, want claude-tmux (per-agent beats global)", got.Candidates[0])
	}
}

func TestResolve_ProfileUsedWhenNoEnv(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	if got.Candidates[0] != "codex-tmux" || got.PrimarySource != "profile.auditor.cli" {
		t.Errorf("got %q/%q, want codex-tmux/profile.auditor.cli", got.Candidates[0], got.PrimarySource)
	}
}

func TestResolve_DefaultWhenAllEmpty(t *testing.T) {
	got := Resolve("auditor", "audit", "auto", nil, nil, nil)
	if got.Candidates[0] != "claude-tmux" || got.PrimarySource != "default" {
		t.Errorf("got %q/%q, want claude-tmux/default", got.Candidates[0], got.PrimarySource)
	}
}

func TestResolve_FallbackDedup(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux", CLIFallback: []string{"codex-tmux", "claude-tmux", "agy-tmux"}}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("candidates=%v, want %v (primary dedup'd)", got.Candidates, want)
	}
}

func TestResolve_FallbackDedupPreservesOrder(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux", CLIFallback: []string{"claude-tmux", "agy-tmux", "claude-tmux"}}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("candidates=%v, want %v", got.Candidates, want)
	}
}

func TestResolve_DefaultTriggers(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux"}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	want := []int{80, 81, 124, 127}
	if !reflect.DeepEqual(got.Triggers, want) {
		t.Errorf("triggers=%v, want %v", got.Triggers, want)
	}
}

func TestResolve_CustomTriggers(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux", CLIFallbackOnExit: []int{80, 127, 81, 2}}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	if !reflect.DeepEqual(got.Triggers, []int{80, 127, 81, 2}) {
		t.Errorf("triggers=%v, want operator-extended list", got.Triggers)
	}
}

func TestResolve_TrimsWhitespace(t *testing.T) {
	prof := &profiles.Profile{CLI: "codex-tmux", CLIFallback: []string{"  claude-tmux\n", "\tagy-tmux ", "  "}}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	want := []string{"codex-tmux", "claude-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("candidates=%v, want %v (whitespace stripped, empty rejected)", got.Candidates, want)
	}
}

func TestPlan_TriggersFallback(t *testing.T) {
	p := Plan{Triggers: []int{80, 127}}
	cases := []struct {
		exit int
		want bool
	}{
		{0, false}, {80, true}, {81, false}, {127, true}, {2, false}, {1, false},
	}
	for _, tc := range cases {
		if got := p.TriggersFallback(tc.exit); got != tc.want {
			t.Errorf("TriggersFallback(%d)=%v, want %v", tc.exit, got, tc.want)
		}
	}
}

// --- Probe (ported from runner/cli_probe_test.go behavior) ---

func TestProbe_DemotesMissingBinary(t *testing.T) {
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{80}, Model: "sonnet"}
	// codex missing, claude present.
	lookPath := func(bin string) (string, error) {
		if bin == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", errors.New("not found")
	}
	got := Probe(p, lookPath)
	want := []string{"claude-tmux", "codex-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("candidates=%v, want %v (missing demoted)", got.Candidates, want)
	}
	if got.Model != "sonnet" || got.PrimarySource != p.PrimarySource {
		t.Errorf("Probe dropped non-candidate fields: %+v", got)
	}
}

func TestProbe_AllMissingKeepsOrder(t *testing.T) {
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	got := Probe(p, func(string) (string, error) { return "", errors.New("nope") })
	if !reflect.DeepEqual(got.Candidates, []string{"codex-tmux", "claude-tmux"}) {
		t.Errorf("all-missing should keep order, got %v", got.Candidates)
	}
}

func TestProbe_SingleCandidateUnchanged(t *testing.T) {
	p := Plan{Candidates: []string{"codex-tmux"}}
	got := Probe(p, func(string) (string, error) { return "", errors.New("nope") })
	if !reflect.DeepEqual(got.Candidates, []string{"codex-tmux"}) {
		t.Errorf("single candidate must be unchanged, got %v", got.Candidates)
	}
}

func TestProbe_AllAvailableNoReorder(t *testing.T) {
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"}}
	got := Probe(p, func(string) (string, error) { return "/usr/bin/x", nil })
	if !reflect.DeepEqual(got.Candidates, p.Candidates) {
		t.Errorf("all-present must not reorder, got %v", got.Candidates)
	}
}

func TestProbe_MultipleMissingPreservesRelativeOrder(t *testing.T) {
	// codex + agy missing, claude + ollama present; available keep their order,
	// missing keep theirs, appended after.
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux", "ollama-tmux"}}
	present := map[string]bool{"claude": true, "ollama": true}
	got := Probe(p, func(bin string) (string, error) {
		if present[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", errors.New("missing")
	})
	want := []string{"claude-tmux", "ollama-tmux", "codex-tmux", "agy-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("candidates=%v, want %v (relative order preserved within available/missing)", got.Candidates, want)
	}
}

func TestProbe_UnknownCLIKeepsPosition(t *testing.T) {
	// An unknown driver name (no cliBinaryFor entry) is kept in place rather
	// than demoted — LookupDriver might still resolve it.
	p := Plan{Candidates: []string{"mystery-cli", "claude-tmux"}}
	got := Probe(p, func(bin string) (string, error) {
		if bin == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", errors.New("missing")
	})
	if !reflect.DeepEqual(got.Candidates, []string{"mystery-cli", "claude-tmux"}) {
		t.Errorf("unknown CLI should keep position, got %v", got.Candidates)
	}
}

func TestProbe_NilLookPathDefaultsToExecLookPath(t *testing.T) {
	// nil lookPath must not panic — it defaults to exec.LookPath. We can't
	// assert a specific order (depends on host PATH), only that it returns a
	// chain of the same length with the same membership.
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	got := Probe(p, nil)
	if len(got.Candidates) != 2 {
		t.Errorf("nil lookPath: got %d candidates, want 2", len(got.Candidates))
	}
}

// --- Model resolution ---

func TestResolve_ModelEnvBeatsProfileAndDefault(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", ModelTierDefault: "sonnet"}
	env := map[string]string{"EVOLVE_AUDITOR_MODEL": "opus"}
	got := Resolve("auditor", "audit", "auto", env, prof, nil)
	if got.Model != "opus" {
		t.Errorf("Model=%q, want opus (env override beats profile + default)", got.Model)
	}
}

func TestResolve_ModelProfileTierBeatsDefault(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", ModelTierDefault: "sonnet"}
	got := Resolve("auditor", "audit", "auto", nil, prof, nil)
	if got.Model != "sonnet" {
		t.Errorf("Model=%q, want sonnet (profile tier beats default 'auto')", got.Model)
	}
}

func TestResolve_ModelDefaultWhenNoEnvNoProfile(t *testing.T) {
	got := Resolve("auditor", "audit", "opus", nil, nil, nil)
	if got.Model != "opus" {
		t.Errorf("Model=%q, want opus (the supplied default)", got.Model)
	}
}

func TestResolve_AutoExpandedViaSeam(t *testing.T) {
	gotRole := ""
	autoExpand := func(role string) (string, bool) {
		gotRole = role
		return "claude-opus-4-7", true
	}
	// defaultModel "auto" + no profile tier → "auto" → expander.
	got := Resolve("auditor", "audit", "auto", nil, nil, autoExpand)
	if got.Model != "claude-opus-4-7" {
		t.Errorf("Model=%q, want claude-opus-4-7 (auto expanded)", got.Model)
	}
	if gotRole != "audit" {
		t.Errorf("autoExpand role=%q, want audit (phase, not agent)", gotRole)
	}
}

func TestResolve_AutoSeamFailureKeepsAuto(t *testing.T) {
	autoExpand := func(role string) (string, bool) { return "", false }
	got := Resolve("auditor", "audit", "auto", nil, nil, autoExpand)
	if got.Model != "auto" {
		t.Errorf("Model=%q, want auto unchanged when expander returns ok=false", got.Model)
	}
}

func TestResolve_NonAutoModelNotExpanded(t *testing.T) {
	called := false
	autoExpand := func(role string) (string, bool) { called = true; return "x", true }
	prof := &profiles.Profile{CLI: "claude-tmux", ModelTierDefault: "sonnet"}
	got := Resolve("auditor", "audit", "auto", nil, prof, autoExpand)
	if got.Model != "sonnet" {
		t.Errorf("Model=%q, want sonnet", got.Model)
	}
	if called {
		t.Error("autoExpand must not be called when model is already concrete (sonnet)")
	}
}
