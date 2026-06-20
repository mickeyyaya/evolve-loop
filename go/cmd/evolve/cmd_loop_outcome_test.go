package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGCHook_InvalidModeSkipped verifies that an unrecognized gc.mode value
// logs a WARN message to stderr and returns without running gc.Plan (the retain
// behavior from the former EVOLVE_GC env handler is preserved post-migration).
func TestRunGCHook_InvalidModeSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir evolveDir: %v", err)
	}

	// Write a policy.json with an invalid gc.mode value.
	pol := map[string]any{
		"gc": map[string]any{"mode": "banana"},
	}
	raw, _ := json.Marshal(pol)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), raw, 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	cfg := loopConfig{EvolveDir: evolveDir}
	workspace := filepath.Join(dir, "workspace")
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	got := buf.String()
	if !strings.Contains(got, "[gc] WARN") {
		t.Errorf("expected WARN in stderr for invalid gc.mode, got: %q", got)
	}
	// Ensure no gc-shadow-manifest.json was written (gc.Plan was not called).
	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); err == nil {
		t.Error("gc-shadow-manifest.json must not be written for an invalid gc.mode")
	}
}
