package baseattr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func commitFiles(t *testing.T, r *gittest.Repo, msg string, files map[string]string) string {
	t.Helper()
	for rel, text := range files {
		target := filepath.Join(r.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", msg)
	return r.Git("rev-parse", "HEAD")
}

func attributorOn(t *testing.T, r *gittest.Repo, run Runner) Attributor {
	t.Helper()
	return Attributor{Git: gitexec.Default(r.Dir), Run: run, MainRef: "main", Scratch: t.TempDir()}
}

func worktreeCount(r *gittest.Repo) int {
	n := 0
	for _, line := range strings.Split(r.Git("worktree", "list", "--porcelain"), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}
