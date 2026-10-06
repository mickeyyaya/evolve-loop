package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileRouter(t *testing.T, root string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, _, err := cliroute.Build(cliroute.Setup{Policy: pol, Profiles: profiles.NewFromDir(dir),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func TestFailureAdvisor_ComesFromTheTable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(map[string]any{"name": "failure-advisor", "cli": "codex-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(dir, "failure-advisor.json"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	if got := failureAdvisorCLI(root, profileRouter(t, root, policy.Policy{CLIRouting: &block})); got != "agy-tmux" {
		t.Fatalf("the failure advisor launches the table's primary, not the profile's codex: %q", got)
	}
	if got := failureAdvisorCLI(root, profileRouter(t, root, policy.Policy{})); got != "codex-tmux" {
		t.Fatalf("with no block the profile's cli stays: %q", got)
	}
}

func TestFailureAdvisorOpts_ResolvesProfileCLI(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(map[string]any{"name": "failure-advisor", "cli": "codex-tmux", "model_tier_default": "deep"})
	if err := os.WriteFile(filepath.Join(dir, "failure-advisor.json"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := len(failureAdvisorOpts(root, profileRouter(t, root, policy.Policy{}))); got != 1 {
		t.Fatalf("failureAdvisorOpts = %d options, want 1 (WithFailureAdvisorCLI from the profile)", got)
	}
}

func TestFailureAdvisorOpts_AbsentProfileFailsOpen(t *testing.T) {
	empty := t.TempDir()
	// Point every resolvellm fallback at the empty tree. failureAdvisorOpts pins
	// GitRoot; the cwd fallback would otherwise read the real repo's profile.
	t.Setenv("EVOLVE_PROJECT_ROOT", empty)
	t.Setenv("EVOLVE_PLUGIN_ROOT", empty)
	if got := len(failureAdvisorOpts(empty, profileRouter(t, empty, policy.Policy{}))); got != 0 {
		t.Fatalf("failureAdvisorOpts = %d options on an empty tree, want 0 (compiled default keeps working)", got)
	}
}
