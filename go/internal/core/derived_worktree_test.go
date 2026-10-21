package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestDerivedWorktree_RunsGitInTheWorktreeAndChecksMarkersAgainstHEAD(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "a\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "a")

	w := DerivedWorktree(repo.Dir)

	out, code, err := w.Git(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil || code != 0 || !strings.HasSuffix(strings.TrimSpace(out), strings.TrimPrefix(repo.Dir, "/private")) {
		t.Fatalf("Git = (%q, %d, %v), want the worktree root %s", out, code, err, repo.Dir)
	}
	if w.Base != "HEAD" || w.Run == nil {
		t.Errorf("Base = %q, Run set = %v; the recovery checks the regenerated files against HEAD", w.Base, w.Run != nil)
	}
}
