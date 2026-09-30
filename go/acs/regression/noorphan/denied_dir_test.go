//go:build acs

package noorphan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindOrphanScripts_SkipsASandboxDeniedDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits — nothing to deny")
	}
	root := t.TempDir()
	denied := filepath.Join(root, "docs", "private")
	if err := os.MkdirAll(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "hidden.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })

	offenders, err := findOrphanScripts(root)
	if err != nil {
		t.Fatalf("a permission-denied directory is skipped, not a scan failure: %v", err)
	}
	if len(offenders) != 0 {
		t.Fatalf("nothing outside the denied directory → no offenders, got %v", offenders)
	}
}
