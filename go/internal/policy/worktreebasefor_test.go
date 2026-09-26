package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeBaseFor_LoadsFromDisk(t *testing.T) {
	dir := t.TempDir()

	if got := WorktreeBaseFor(dir); got != "" {
		t.Errorf("absent policy.json: WorktreeBaseFor = %q, want \"\"", got)
	}

	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"),
		[]byte(`{"worktree":{"base":"/mnt/fast/wt"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := WorktreeBaseFor(dir); got != "/mnt/fast/wt" {
		t.Errorf("WorktreeBaseFor = %q, want /mnt/fast/wt", got)
	}
}
