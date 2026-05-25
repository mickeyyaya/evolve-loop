// rollback_unit_test.go — seam-injected unit tests for the lowest-
// coverage helpers in rollback.go. The default* functions are
// integration-only (real gh/git/evolve shell-outs); this file targets
// the testable helpers: resolveEvolveBinForRollback + appendLedger.
package rollback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveEvolveBinForRollback_EnvVarSet — EVOLVE_GO_BIN pointing
// at an executable file is preferred over PATH lookup.
func TestResolveEvolveBinForRollback_EnvVarSet(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-evolve")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", binPath)
	got := resolveEvolveBinForRollback(t.TempDir())
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

// TestResolveEvolveBinForRollback_EnvVarNonExecutable_FallsThrough —
// when EVOLVE_GO_BIN points at a non-executable file, fall through.
func TestResolveEvolveBinForRollback_EnvVarNonExecutable_FallsThrough(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "not-exec")
	if err := os.WriteFile(binPath, []byte("x"), 0o644); err != nil { // 0o644 = not exec
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", binPath)
	// Repo root has no go/bin/evolve either; PATH unlikely to have it.
	got := resolveEvolveBinForRollback(t.TempDir())
	if got == binPath {
		t.Error("non-executable env var should be skipped")
	}
}

// TestResolveEvolveBinForRollback_RepoRootCandidate — falls back to
// <repoRoot>/go/bin/evolve when env unset.
func TestResolveEvolveBinForRollback_RepoRootCandidate(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	repo := t.TempDir()
	candidate := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(candidate), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveEvolveBinForRollback(repo); got != candidate {
		t.Errorf("got %q, want %q", got, candidate)
	}
}

// TestResolveEvolveBinForRollback_NotFound_ReturnsEmpty — when none of
// the lookup paths resolve, returns empty string. (PATH is unpredictable
// in CI; we use a deliberately unlikely repoRoot.)
func TestResolveEvolveBinForRollback_NotFound_ReturnsEmpty(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "/nonexistent/path")
	if got := resolveEvolveBinForRollback("/no/such/repo/root"); got != "" {
		t.Errorf("got %q, want empty (no binary anywhere)", got)
	}
}

// TestAppendLedger_HappyPath — file is created with the line + newline.
func TestAppendLedger_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "ledger.jsonl")
	if err := appendLedger(path, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"a\":1}\n" {
		t.Errorf("got %q", string(b))
	}
}

// TestAppendLedger_AppendsMultiple — second call appends, doesn't truncate.
func TestAppendLedger_AppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	_ = appendLedger(path, []byte(`one`))
	_ = appendLedger(path, []byte(`two`))
	b, _ := os.ReadFile(path)
	if string(b) != "one\ntwo\n" {
		t.Errorf("got %q, want 'one\\ntwo\\n'", string(b))
	}
}

// TestAppendLedger_MkdirFailure_ReturnsError — when parent path is a
// file, MkdirAll fails and the error surfaces.
func TestAppendLedger_MkdirFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Try to create <blocker>/child/ledger.jsonl — blocker is a file
	path := filepath.Join(blocker, "child", "ledger.jsonl")
	err := appendLedger(path, []byte("data"))
	if err == nil {
		t.Error("expected mkdir error when parent is a file")
	}
	if !strings.Contains(err.Error(), "blocker") && !strings.Contains(err.Error(), "child") {
		// loose check: error path should reference the failing dir
	}
}
