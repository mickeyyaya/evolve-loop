package main

// cmd_cycle_failureadvisor_test.go — the composition-root half of the
// failure-advisor identity wiring (2026-08-26 deep-tier review HIGH: the
// advisor hardcoded claude-tmux/opus and never read its profile; dormant while
// PhaseRecovery=shadow but wrong the moment it flips to enforce). The option's
// effect on the dispatched BridgeRequest is pinned in core
// (apicover_misc_test.go); THIS pins that the root actually resolves the
// tracked profile into that option.

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
	// Isolate every resolvellm fallback rung: env roots point at the empty
	// tree, and failureAdvisorOpts itself pins GitRoot to projectRoot (the
	// process-cwd git-root fallback would otherwise resolve the REAL repo's
	// tracked profile and make this test — and worktree production setups —
	// read the wrong tree).
	t.Setenv("EVOLVE_PROJECT_ROOT", empty)
	t.Setenv("EVOLVE_PLUGIN_ROOT", empty)
	if got := len(failureAdvisorOpts(empty)); got != 0 {
		t.Fatalf("failureAdvisorOpts = %d options on an empty tree, want 0 (compiled default keeps working)", got)
	}
}
