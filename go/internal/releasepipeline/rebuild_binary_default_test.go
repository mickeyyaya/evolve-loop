package releasepipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultRebuildBinary_NonDryRun_BadSourceDir: when dryRun=false and the
// go toolchain is on PATH but the source dir (<repoRoot>/go) has no Go source
// (or cmd/evolve doesn't exist), `go build` exits non-zero and the function
// returns a wrapped error mentioning "go build".
func TestDefaultRebuildBinary_NonDryRun_BadSourceDir(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	// t.TempDir() has no Go source — go build will fail.
	dir := t.TempDir()
	err := defaultRebuildBinary(dir, "9.9.9", false)
	if err == nil {
		t.Fatal("defaultRebuildBinary with empty source dir: want error, got nil")
	}
	if !strings.Contains(err.Error(), "go build") {
		t.Errorf("error = %q, want mention of 'go build'", err.Error())
	}
}

// TestDefaultRebuildBinary_NonDryRun_RealRepo: when dryRun=false and a copy of
// the current Go module is provided, `go build -o evolve ./cmd/evolve` succeeds
// and the binary is written inside an isolated temporary repository.
//
// This is an integration test — it runs a real `go build`. It is skipped when
// the go toolchain is unavailable.
func TestDefaultRebuildBinary_NonDryRun_RealRepo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	sourceRoot := findRepoRoot(t)
	repoRoot := initTempRepoWithTag(t, "v0.0.1")
	// Copy current source (including uncommitted edits), embeds and vendored
	// dependencies. Build only in the copy: restoring the source binary after
	// a build still lets concurrent ship commands stage it while the test runs.
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
	// The ldflags version stamp (release-verify's acceptance): the rebuilt
	// binary must self-report the target. Without ldflags injection a plain
	// `go build` reports the VCS revision and release-verify fails on every
	// release.
	out, err := exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		t.Errorf("rebuilt binary --version: %v (%s)", err, out)
	} else if !strings.Contains(string(out), "9.9.9") {
		t.Errorf("rebuilt binary --version = %q, want it to report the target 9.9.9 (ldflags stamp)", strings.TrimSpace(string(out)))
	}
}
