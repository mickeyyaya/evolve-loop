package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func writePolicyJSON(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
