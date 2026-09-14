//go:build acs

package flagreaders

import (
	"os"
	"path/filepath"
	"testing"
)

// The audit phase's sandbox denies docs/private; the repo-wide shell scan
// must skip a denied directory instead of failing the gate (cycles 1676/1679).
func TestScanTextTree_SkipsASandboxDeniedDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits — nothing to deny")
	}
	root := t.TempDir()
	denied := filepath.Join(root, "docs", "private")
	if err := os.MkdirAll(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "hidden.sh"), []byte("echo $EVOLVE_NOT_A_REAL_FLAG\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })

	orphans := map[string][]string{}
	if err := scanTextTree(root, map[string]bool{".sh": true}, func(string) bool { return false }, orphans); err != nil {
		t.Fatalf("a permission-denied directory is skipped, not a scan failure: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("the denied file is never read → no orphans, got %v", orphans)
	}
}
