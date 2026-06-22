package releasepipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// makeExecutable writes a minimal shell script that exits 0 and sets
// the executable bit, so os.Stat().Mode()&0o111 != 0.
func makeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("makeExecutable %s: %v", name, err)
	}
	return path
}

// makeNonExecutable writes a file without the executable bit.
func makeNonExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("makeNonExecutable %s: %v", name, err)
	}
	return path
}

// === resolveEvolveBin — EVOLVE_GO_BIN env var present and executable =========

// TestResolveEvolveBin_EnvVarExecutable: when EVOLVE_GO_BIN points to an
// executable file, resolveEvolveBin returns that exact path.
func TestResolveEvolveBin_EnvVarExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := makeExecutable(t, dir, "evolve")
	t.Setenv("EVOLVE_GO_BIN", bin)

	got := resolveEvolveBin(dir)
	if got != bin {
		t.Errorf("resolveEvolveBin = %q, want %q (EVOLVE_GO_BIN path)", got, bin)
	}
}

// TestResolveEvolveBin_EnvVarNonExecutable: when EVOLVE_GO_BIN points to a
// non-executable file, the env-var branch does NOT return that path;
// the function falls through to the next candidate.
func TestResolveEvolveBin_EnvVarNonExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := makeNonExecutable(t, dir, "evolve")
	t.Setenv("EVOLVE_GO_BIN", bin)

	// Make sure neither the repo-relative path nor PATH has a real evolve binary.
	// Use a fresh repoRoot that has no go/bin/evolve.
	emptyRoot := t.TempDir()
	t.Setenv("PATH", "") // clear PATH so exec.LookPath can't find it either

	got := resolveEvolveBin(emptyRoot)
	// Should be empty because the env-var file is not executable, no bin at
	// <emptyRoot>/go/bin/evolve, and PATH is cleared.
	if got != "" {
		t.Errorf("resolveEvolveBin with non-executable EVOLVE_GO_BIN = %q, want empty", got)
	}
}

// TestResolveEvolveBin_EnvVarNotSet_RepoBin: when EVOLVE_GO_BIN is unset but
// <repoRoot>/go/bin/evolve is an executable, resolveEvolveBin returns that path.
func TestResolveEvolveBin_EnvVarNotSet_RepoBin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "") // explicitly unset

	// Create <dir>/go/bin/evolve as executable.
	binDir := filepath.Join(dir, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(binDir, "evolve")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := resolveEvolveBin(dir)
	if got != binPath {
		t.Errorf("resolveEvolveBin = %q, want %q (<repoRoot>/go/bin/evolve)", got, binPath)
	}
}

// TestResolveEvolveBin_EnvVarNotSet_RepoBinNonExecutable: when EVOLVE_GO_BIN
// is unset and <repoRoot>/go/bin/evolve exists but is not executable, it falls
// through to PATH lookup.
func TestResolveEvolveBin_EnvVarNotSet_RepoBinNonExecutable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "") // no evolve on PATH either

	binDir := filepath.Join(dir, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	makeNonExecutable(t, binDir, "evolve")

	got := resolveEvolveBin(dir)
	if got != "" {
		t.Errorf("resolveEvolveBin with non-executable repo bin = %q, want empty", got)
	}
}

// TestResolveEvolveBin_AllMissing: when EVOLVE_GO_BIN is unset, no binary at
// <repoRoot>/go/bin/evolve, and PATH is empty, resolveEvolveBin returns "".
func TestResolveEvolveBin_AllMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")

	got := resolveEvolveBin(dir)
	if got != "" {
		t.Errorf("resolveEvolveBin with nothing on PATH = %q, want empty", got)
	}
}

// === resolveEvolveBin — <repoRoot>/go/evolve (rebuild-binary output) =========

// TestResolveEvolveBin_TrackedGoEvolve: the release's rebuild-binary step builds
// to <repoRoot>/go/evolve; resolveEvolveBin must find it when go/bin/evolve is
// absent. Regression guard for the v18.2.0 release failure, where ship reported
// "binary not found" one step after rebuild-binary produced the binary there.
func TestResolveEvolveBin_TrackedGoEvolve(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "") // no ambient evolve on PATH
	goDir := filepath.Join(dir, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := makeExecutable(t, goDir, "evolve")

	if got := resolveEvolveBin(dir); got != want {
		t.Errorf("resolveEvolveBin = %q, want %q (<repoRoot>/go/evolve)", got, want)
	}
}

// TestResolveEvolveBin_RepoBinBeatsTrackedGoEvolve: precedence — when both
// <repoRoot>/go/bin/evolve and <repoRoot>/go/evolve exist, the gitignored local
// build (go/bin/evolve) wins over the tracked binary.
func TestResolveEvolveBin_RepoBinBeatsTrackedGoEvolve(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	binDir := filepath.Join(dir, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wantBin := makeExecutable(t, binDir, "evolve")
	makeExecutable(t, filepath.Join(dir, "go"), "evolve") // also present, lower precedence

	if got := resolveEvolveBin(dir); got != wantBin {
		t.Errorf("resolveEvolveBin = %q, want %q (go/bin precedence)", got, wantBin)
	}
}

// === defaultRebuildBinary — dryRun=true short-circuits =====================

// TestDefaultRebuildBinary_DryRunIsNoop: dryRun=true must return nil without
// running any go build command (no go toolchain needed).
func TestDefaultRebuildBinary_DryRunIsNoop(t *testing.T) {
	// An empty TempDir has no go source; if build were attempted it would fail.
	err := defaultRebuildBinary(t.TempDir(), "9.9.9", true)
	if err != nil {
		t.Errorf("defaultRebuildBinary(dryRun=true) = %v, want nil", err)
	}
}

// NOTE: the former TestDefaultFullDryRunPreflight_Script{Missing,NotExecutable,
// Fails} tests were removed in ADR-0062/T1.3. They asserted the deleted bash
// shell-out (legacy/scripts/release/full-dry-run.sh); the Go-native replacement
// is covered by TestDefaultFullDryRunPreflight_NoDeadScript.
