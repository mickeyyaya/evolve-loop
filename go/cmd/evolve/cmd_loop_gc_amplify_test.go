package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAmplifyGCNoPolicyFileDefaultsToShadow(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	workspace := t.TempDir()

	runGCHook(loopConfig{EvolveDir: evolveDir}, workspace, os.Stderr)

	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); err != nil {
		t.Fatalf("absent policy.json must default to shadow and publish the manifest, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "workspace-gc-manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("unset ProjectRoot must skip the worktree sweep, stat err=%v", err)
	}
}

func TestAmplifyGCShadowContinuesAfterPolicyLoadError(t *testing.T) {
	t.Parallel()
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
	if _, err := os.Stat(filepath.Join(workspace, "gc-shadow-manifest.json")); err != nil {
		t.Fatalf("unreadable policy → shadow; manifest must still be published, stat err=%v", err)
	}
	if fi, err := os.Stat(filepath.Join(evolveDir, "policy.json")); err != nil || !fi.IsDir() {
		t.Fatalf("shadow must not mutate the tree: policy.json placeholder changed, fi=%v err=%v", fi, err)
	}
}

func TestAmplifyGCEnforceReturnsWhenManifestPathCannotBeWritten(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
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
