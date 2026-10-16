package marketplacepoll

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultReleaseSh_IgnoresLegacyScript(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "legacy", "scripts", "utility")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release.sh"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DefaultReleaseSh(repo, "1.0.0"); err != nil {
		t.Errorf("DefaultReleaseSh must be a Go no-op and never run a legacy release.sh; got %v", err)
	}
}
