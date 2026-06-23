package marketplacepoll

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultReleaseSh_IgnoresLegacyScript guards the T1.7 fix: DefaultReleaseSh
// is a Go no-op and must NEVER execute a legacy bash release.sh. The script was
// removed in v12 (script→Go migration); the cache-refresh side-effect is
// obsolete and release consistency is covered by internal/releaseconsistency.
// Even a stray executable legacy/scripts/utility/release.sh that exits 1 must be
// ignored (return nil), proving no bash shell-out remains.
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
