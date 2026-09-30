package releasepipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func makeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("makeExecutable %s: %v", name, err)
	}
	return path
}

func makeNonExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("makeNonExecutable %s: %v", name, err)
	}
	return path
}

func TestResolveEvolveBin_EnvVarExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := makeExecutable(t, dir, "evolve")
	t.Setenv("EVOLVE_GO_BIN", bin)

	got := resolveEvolveBin(dir)
	if got != bin {
		t.Errorf("resolveEvolveBin = %q, want %q (EVOLVE_GO_BIN path)", got, bin)
	}
}

func TestResolveEvolveBin_EnvVarNonExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := makeNonExecutable(t, dir, "evolve")
	t.Setenv("EVOLVE_GO_BIN", bin)

	emptyRoot := t.TempDir()
	t.Setenv("PATH", "")

	got := resolveEvolveBin(emptyRoot)
	if got != "" {
		t.Errorf("resolveEvolveBin with non-executable EVOLVE_GO_BIN = %q, want empty", got)
	}
}

func TestResolveEvolveBin_EnvVarNotSet_RepoBin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")

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

func TestResolveEvolveBin_EnvVarNotSet_RepoBinNonExecutable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")

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

func TestResolveEvolveBin_AllMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")

	got := resolveEvolveBin(dir)
	if got != "" {
		t.Errorf("resolveEvolveBin with nothing on PATH = %q, want empty", got)
	}
}

func TestResolveEvolveBin_TrackedGoEvolve(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")
	goDir := filepath.Join(dir, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := makeExecutable(t, goDir, "evolve")

	if got := resolveEvolveBin(dir); got != want {
		t.Errorf("resolveEvolveBin = %q, want %q (<repoRoot>/go/evolve)", got, want)
	}
}

func TestResolveEvolveBin_RepoBinBeatsTrackedGoEvolve(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EVOLVE_GO_BIN", "")
	binDir := filepath.Join(dir, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wantBin := makeExecutable(t, binDir, "evolve")
	makeExecutable(t, filepath.Join(dir, "go"), "evolve")

	if got := resolveEvolveBin(dir); got != wantBin {
		t.Errorf("resolveEvolveBin = %q, want %q (go/bin precedence)", got, wantBin)
	}
}

func TestDefaultRebuildBinary_DryRunIsNoop(t *testing.T) {
	repoWithoutGoSource := t.TempDir()
	err := defaultRebuildBinary(repoWithoutGoSource, "9.9.9", true)
	if err != nil {
		t.Errorf("defaultRebuildBinary(dryRun=true) = %v, want nil", err)
	}
}
