package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
	if got := len(failureAdvisorOpts(root)); got != 1 {
		t.Fatalf("failureAdvisorOpts = %d options, want 1 (WithFailureAdvisorCLI from the profile)", got)
	}
}

func TestFailureAdvisorOpts_AbsentProfileFailsOpen(t *testing.T) {
	empty := t.TempDir()
	// Point every resolvellm fallback at the empty tree. failureAdvisorOpts pins
	// GitRoot; the cwd fallback would otherwise read the real repo's profile.
	t.Setenv("EVOLVE_PROJECT_ROOT", empty)
	t.Setenv("EVOLVE_PLUGIN_ROOT", empty)
	if got := len(failureAdvisorOpts(empty)); got != 0 {
		t.Fatalf("failureAdvisorOpts = %d options on an empty tree, want 0 (compiled default keeps working)", got)
	}
}
