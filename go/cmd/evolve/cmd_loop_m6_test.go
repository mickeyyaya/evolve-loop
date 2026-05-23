package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunLoop_LegacyBash_Disabled covers the default path: env unset
// means the Go-native dispatcher runs. Just asserts the rollback
// branch is not taken.
func TestRunLoop_LegacyBash_Disabled(t *testing.T) {
	t.Setenv("EVOLVE_USE_LEGACY_BASH", "")
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fakeStorage{}
	ledger := &fakeLedger{}
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	if strings.Contains(stderr.String(), "EVOLVE_USE_LEGACY_BASH") {
		t.Fatalf("rollback hatch should not fire when env unset; stderr=%q", stderr.String())
	}
}

// TestRunLoop_LegacyBash_DispatcherMissing covers the rollback-hatch
// error branch: env=1, but the archived dispatcher script doesn't
// exist at the expected path → rc=1 + diagnostic. Uses the
// execLegacyBashFn seam so the test doesn't actually exec bash.
func TestRunLoop_LegacyBash_DispatcherMissing(t *testing.T) {
	t.Setenv("EVOLVE_USE_LEGACY_BASH", "1")

	projectRoot := t.TempDir()
	// No archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh in
	// projectRoot → execLegacyBashReal returns rc=1.
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--goal-text", "test",
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1 (dispatcher missing)", rc)
	}
	if !strings.Contains(stderr.String(), "EVOLVE_USE_LEGACY_BASH=1 but") {
		t.Fatalf("stderr should explain the missing-dispatcher error: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh") {
		t.Fatalf("stderr should mention archived path: %q", stderr.String())
	}
}

// TestRunLoop_LegacyBash_SeamCaptured verifies the exec seam fires
// with the correct args + projectRoot, without actually exec'ing.
func TestRunLoop_LegacyBash_SeamCaptured(t *testing.T) {
	t.Setenv("EVOLVE_USE_LEGACY_BASH", "1")

	prev := execLegacyBashFn
	defer func() { execLegacyBashFn = prev }()
	var capturedRoot string
	var capturedArgs []string
	execLegacyBashFn = func(projectRoot string, args []string, _ io.Writer) int {
		capturedRoot = projectRoot
		capturedArgs = args
		return 42 // sentinel
	}

	projectRoot := t.TempDir()
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--goal-text", "test",
		"--cycles", "3",
	}, nil, &stdout, &stderr)
	if rc != 42 {
		t.Fatalf("rc=%d want 42 (seam sentinel); stderr=%q", rc, stderr.String())
	}
	if capturedRoot != projectRoot {
		t.Fatalf("seam got projectRoot=%q want %q", capturedRoot, projectRoot)
	}
	// args must include --goal-text + --cycles for the bash dispatcher
	// to re-parse them.
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--goal-text") || !strings.Contains(joined, "test") {
		t.Fatalf("seam args missing --goal-text: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--cycles") || !strings.Contains(joined, "3") {
		t.Fatalf("seam args missing --cycles 3: %v", capturedArgs)
	}
}

// TestLegacyDispatcherPath covers the path resolver helper directly.
func TestLegacyDispatcherPath(t *testing.T) {
	t.Parallel()
	got := legacyDispatcherPath("/p")
	want := filepath.Join("/p", "archive", "legacy", "scripts", "dispatch", "evolve-loop-dispatch.sh")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
