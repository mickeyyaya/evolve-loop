package releasepipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEvolveBin_PathLookup(t *testing.T) {
	binDir := t.TempDir()
	makeExecutable(t, binDir, "evolve")

	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	emptyRoot := t.TempDir()
	got := resolveEvolveBin(emptyRoot)

	want := filepath.Join(binDir, "evolve")
	if got != want {
		t.Errorf("resolveEvolveBin via PATH = %q, want %q", got, want)
	}
}

func TestDefaultRebuildBinary_NonDryRun_GoNotOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	err := defaultRebuildBinary(t.TempDir(), "9.9.9", false)
	if err == nil {
		t.Fatal("defaultRebuildBinary with no go on PATH: want error, got nil")
	}
	if !strings.Contains(err.Error(), "go toolchain") {
		t.Errorf("error = %q, want mention of 'go toolchain'", err.Error())
	}
}
