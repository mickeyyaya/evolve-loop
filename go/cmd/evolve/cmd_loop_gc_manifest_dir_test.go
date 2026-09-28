package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestC1735_001_GCManifestDirIsBatchOwnedNotCycleWorkspace(t *testing.T) {
	evolveDir := filepath.FromSlash("/tmp/example-evolve-dir")

	got := gcManifestDir(evolveDir)
	if want := filepath.Join(evolveDir, "gc"); got != want {
		t.Errorf("gcManifestDir(%q) = %q, want %q", evolveDir, got, want)
	}
	if runs := filepath.Join(evolveDir, "runs"); strings.HasPrefix(got, runs) {
		t.Errorf("gcManifestDir(%q) = %q must not sit under the cycle run dirs %q", evolveDir, got, runs)
	}
}
