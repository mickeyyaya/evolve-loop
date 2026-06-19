package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmplifyGCEmptyEnvIsOff(t *testing.T) {
	// No policy.json with gc.mode → gcPol.Mode="" → treated as "off".
	evolveDir := t.TempDir()
	workspace := t.TempDir()

	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, os.Stderr)

	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("absent gc.mode should behave like off and write no manifest, stat err=%v", err)
	}
}

// TestAmplifyGCShadowContinuesAfterPolicyLoadError verifies that when policy.json
// is unreadable, runGCHook logs a WARN and returns (no manifest). After the
// EVOLVE_GC→gc.Policy.Mode migration the mode lives in policy.json, so an
// unreadable policy means mode="" (off) — safe default.
func TestAmplifyGCShadowContinuesAfterPolicyLoadError(t *testing.T) {
	evolveDir := t.TempDir()
	workspace := t.TempDir()
	// policy.json is a directory — unreadable, triggers WARN.
	if err := os.Mkdir(filepath.Join(evolveDir, "policy.json"), 0o755); err != nil {
		t.Fatalf("create unreadable policy placeholder: %v", err)
	}

	var stderr strings.Builder
	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, &stderr)

	if !strings.Contains(strings.ToLower(stderr.String()), "policy") {
		t.Fatalf("policy load failure should be logged, stderr=%q", stderr.String())
	}
	// Mode defaults to "off" when policy is unreadable — no manifest written.
	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("unreadable policy → mode=off; manifest must not be written, stat err=%v", err)
	}
}

func TestAmplifyGCEnforceReturnsWhenManifestPathCannotBeWritten(t *testing.T) {
	evolveDir := t.TempDir()
	// Write a policy.json with mode=enforce so the hook proceeds to the write step.
	pol := `{"gc":{"mode":"enforce"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	workspaceParent := t.TempDir()
	workspace := filepath.Join(workspaceParent, "not-a-directory")
	if err := os.WriteFile(workspace, []byte("workspace path is a file"), 0o644); err != nil {
		t.Fatalf("create file workspace: %v", err)
	}

	var stderr strings.Builder
	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, &stderr)

	if !strings.Contains(strings.ToLower(stderr.String()), "gc") {
		t.Fatalf("manifest write failure should be logged as a gc warning, stderr=%q", stderr.String())
	}
	if got, err := os.ReadFile(workspace); err != nil || string(got) != "workspace path is a file" {
		t.Fatalf("hook should not replace a non-directory workspace path, got=%q err=%v", got, err)
	}
}
