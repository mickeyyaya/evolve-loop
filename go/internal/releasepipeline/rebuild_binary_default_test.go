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
	err := defaultRebuildBinary(dir, false)
	if err == nil {
		t.Fatal("defaultRebuildBinary with empty source dir: want error, got nil")
	}
	if !strings.Contains(err.Error(), "go build") {
		t.Errorf("error = %q, want mention of 'go build'", err.Error())
	}
}

// TestDefaultRebuildBinary_NonDryRun_RealRepo: when dryRun=false and the real
// repo root is provided, `go build -o evolve ./cmd/evolve` succeeds and the
// binary is written to <repoRoot>/go/evolve.
//
// This is an integration test — it runs a real `go build`. It is skipped when
// the go toolchain is unavailable. Build time is bounded by the existing binary
// cache, typically < 5 s.
func TestDefaultRebuildBinary_NonDryRun_RealRepo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	repoRoot := findRepoRoot(t)

	// defaultRebuildBinary writes <repoRoot>/go/evolve in place. Snapshot the
	// tracked binary and restore it afterward so this integration test never
	// leaves the committed artifact dirty — otherwise every `go test` (commit
	// gate, CI, make test) silently mutates go/evolve, which then trips the ship
	// gate's self-SHA check and pollutes unrelated commits.
	binPath := filepath.Join(repoRoot, "go", "evolve")
	origInfo, statErr := os.Stat(binPath)
	orig, readErr := os.ReadFile(binPath)
	t.Cleanup(func() {
		if readErr != nil {
			return // nothing to restore (binary absent before the test)
		}
		mode := os.FileMode(0o755)
		if statErr == nil {
			mode = origInfo.Mode() // preserve the committed file's exact mode
		}
		if err := os.WriteFile(binPath, orig, mode); err != nil {
			t.Errorf("restore %s: %v", binPath, err)
		}
	})

	if err := defaultRebuildBinary(repoRoot, false); err != nil {
		t.Errorf("defaultRebuildBinary on real repo: %v", err)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Errorf("expected rebuilt binary at %s: %v", binPath, err)
	}
}
