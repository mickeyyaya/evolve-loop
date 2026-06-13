package releasepipeline

import (
	"strings"
	"testing"
)

func TestDefaultReleaseVerify_RelativeRepoRoot(t *testing.T) {
	err := defaultReleaseVerify("relative/repo", "1.2.3", "deadbeef")
	if err == nil {
		t.Fatal("defaultReleaseVerify with relative repoRoot returned nil error")
	}
	if !strings.Contains(err.Error(), "repoRoot must be absolute") {
		t.Fatalf("defaultReleaseVerify error = %q, want absolute-path guard", err)
	}
}

func TestDefaultReleaseVerify_MissingBinaryOnDisk(t *testing.T) {
	err := defaultReleaseVerify(t.TempDir(), "1.2.3", "deadbeef")
	if err == nil {
		t.Fatal("defaultReleaseVerify with missing go/evolve returned nil error")
	}
	if !strings.Contains(err.Error(), "tracked binary missing on disk") {
		t.Fatalf("defaultReleaseVerify error = %q, want missing tracked binary", err)
	}
}

func TestDefaultShip_BinaryNotFound(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", t.TempDir())

	_, err := defaultShip(t.TempDir(), "release: test", "notes")
	if err == nil {
		t.Fatal("defaultShip with no evolve binary returned nil error")
	}
	if !strings.Contains(err.Error(), "evolve binary not found") {
		t.Fatalf("defaultShip error = %q, want binary-not-found guard", err)
	}
}
