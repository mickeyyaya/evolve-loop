package releasepipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultRebuildBinary_NonDryRun_BadSourceDir(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	repoWithoutGoSource := t.TempDir()
	err := defaultRebuildBinary(repoWithoutGoSource, "9.9.9", false)
	if err == nil {
		t.Fatal("defaultRebuildBinary with empty source dir: want error, got nil")
	}
	if !strings.Contains(err.Error(), "go build") {
		t.Errorf("error = %q, want mention of 'go build'", err.Error())
	}
}

func TestDefaultRebuildBinary_NonDryRun_RealRepo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	sourceRoot := findRepoRoot(t)
	repoRoot := initTempRepoWithTag(t, "v0.0.1")
	if err := os.CopyFS(filepath.Join(repoRoot, "go"), os.DirFS(filepath.Join(sourceRoot, "go"))); err != nil {
		t.Fatalf("copy current Go module: %v", err)
	}
	binPath := filepath.Join(repoRoot, "go", "evolve")
	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove copied build output: %v", err)
	}

	if err := defaultRebuildBinary(repoRoot, "9.9.9", false); err != nil {
		t.Errorf("defaultRebuildBinary on real repo: %v", err)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Errorf("expected rebuilt binary at %s: %v", binPath, err)
	}
	out, err := exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		t.Errorf("rebuilt binary --version: %v (%s)", err, out)
	} else if !strings.Contains(string(out), "9.9.9") {
		t.Errorf("rebuilt binary --version = %q, want it to report the target 9.9.9 (ldflags stamp)", strings.TrimSpace(string(out)))
	}
}
