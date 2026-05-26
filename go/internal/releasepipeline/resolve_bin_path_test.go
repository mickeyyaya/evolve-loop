package releasepipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveEvolveBin_PathLookup: when EVOLVE_GO_BIN is unset and no
// binary exists at <repoRoot>/go/bin/evolve, but a fake "evolve" executable
// is on PATH, resolveEvolveBin returns the PATH-resolved binary.
func TestResolveEvolveBin_PathLookup(t *testing.T) {
	// Create a fake "evolve" in a temp dir and prepend it to PATH.
	binDir := t.TempDir()
	makeExecutable(t, binDir, "evolve")

	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	// Use an empty repoRoot so <repoRoot>/go/bin/evolve doesn't exist.
	emptyRoot := t.TempDir()
	got := resolveEvolveBin(emptyRoot)

	want := filepath.Join(binDir, "evolve")
	if got != want {
		t.Errorf("resolveEvolveBin via PATH = %q, want %q", got, want)
	}
}

// TestDefaultRebuildBinary_NonDryRun_GoNotOnPath: when dryRun=false and
// the go toolchain is not on PATH, defaultRebuildBinary returns an error
// mentioning "go toolchain".
func TestDefaultRebuildBinary_NonDryRun_GoNotOnPath(t *testing.T) {
	// Clear PATH so exec.LookPath("go") fails.
	t.Setenv("PATH", "")

	err := defaultRebuildBinary(t.TempDir(), false)
	if err == nil {
		t.Fatal("defaultRebuildBinary with no go on PATH: want error, got nil")
	}
	if !containsStr(err.Error(), "go toolchain") {
		t.Errorf("error = %q, want mention of 'go toolchain'", err.Error())
	}
}
