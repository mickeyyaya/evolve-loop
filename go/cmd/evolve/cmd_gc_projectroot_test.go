package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestResolveGCProjectRoot_RefusesEmptyMutatingRun(t *testing.T) {
	var stderr bytes.Buffer
	root, code, ok := resolveGCProjectRoot("", false, &stderr)

	if ok {
		t.Fatalf("resolveGCProjectRoot ok=true for an empty --project-root on a mutating run; want refusal")
	}
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if root != "" {
		t.Errorf("root = %q, want empty on refusal", root)
	}
	if !strings.Contains(stderr.String(), "mutating run refused: --project-root must be explicitly set") {
		t.Errorf("stderr missing refusal message, got: %s", stderr.String())
	}
}

func TestResolveGCProjectRoot_FallsBackToCwdOnDryRun(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	var stderr bytes.Buffer
	root, code, ok := resolveGCProjectRoot("", true, &stderr)

	if !ok {
		t.Fatalf("resolveGCProjectRoot ok=false for an empty --project-root on a dry-run; want cwd fallback, stderr=%s", stderr.String())
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if root != cwd {
		t.Errorf("root = %q, want cwd %q", root, cwd)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr not empty on dry-run cwd fallback: %s", stderr.String())
	}
}

func TestResolveGCProjectRoot_PassesThroughExplicitValue(t *testing.T) {
	var stderr bytes.Buffer
	root, code, ok := resolveGCProjectRoot("/explicit/root", false, &stderr)

	if !ok {
		t.Fatalf("resolveGCProjectRoot ok=false for an explicit non-empty --project-root")
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if root != "/explicit/root" {
		t.Errorf("root = %q, want passthrough of the explicit value", root)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr not empty on passthrough: %s", stderr.String())
	}
}

func TestResolveGCProjectRoot_ExplicitValueIgnoresDryRunFlag(t *testing.T) {
	var stderr bytes.Buffer
	root, code, ok := resolveGCProjectRoot("/explicit/root", true, &stderr)

	if !ok {
		t.Fatalf("resolveGCProjectRoot ok=false for an explicit non-empty --project-root under --dry-run")
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if root != "/explicit/root" {
		t.Errorf("root = %q, want passthrough of the explicit value regardless of dryRun", root)
	}
}
