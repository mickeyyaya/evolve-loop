package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestAmplifyGCEmptyEnvIsOff(t *testing.T) {
	t.Setenv("EVOLVE_GC", "")

	evolveDir := t.TempDir()
	workspace := t.TempDir()

	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, os.Stderr)

	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("empty EVOLVE_GC should behave like off and write no manifest, stat err=%v", err)
	}
}

func TestAmplifyGCShadowContinuesAfterPolicyLoadError(t *testing.T) {
	t.Setenv("EVOLVE_GC", "shadow")

	evolveDir := t.TempDir()
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(evolveDir, "policy.json"), 0o755); err != nil {
		t.Fatalf("create unreadable policy placeholder: %v", err)
	}

	var stderr strings.Builder
	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, &stderr)

	if !strings.Contains(strings.ToLower(stderr.String()), "policy") {
		t.Fatalf("policy load failure should be logged, stderr=%q", stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(workspace, "gc-shadow-manifest.json"))
	if err != nil {
		t.Fatalf("shadow mode should still write a manifest after policy load failure: %v", err)
	}

	var manifest gc.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("shadow manifest should be valid gc.Manifest JSON: %v\n%s", err, data)
	}
	if len(manifest.Items) != 0 {
		t.Fatalf("missing runs directory should produce an empty manifest, got %+v", manifest.Items)
	}
}

func TestAmplifyGCEnforceReturnsWhenManifestPathCannotBeWritten(t *testing.T) {
	t.Setenv("EVOLVE_GC", "enforce")

	evolveDir := t.TempDir()
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
