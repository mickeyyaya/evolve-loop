package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGCHook_InvalidModeSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir evolveDir: %v", err)
	}

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
	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); err == nil {
		t.Error("gc-shadow-manifest.json must not be written for an invalid gc.mode")
	}
}
