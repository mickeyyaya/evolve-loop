package gittest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAt_InitsAWorkTreeInTheGivenDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	r := At(t, dir)

	if r.Dir != dir {
		t.Fatalf("At(dir).Dir = %q, want %q", r.Dir, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("At did not init a repo in %s: %v", dir, err)
	}
	if got := strings.TrimSpace(r.Git("config", "user.name")); got != "gittest" {
		t.Errorf("user.name = %q, want the fixture identity", got)
	}
}
