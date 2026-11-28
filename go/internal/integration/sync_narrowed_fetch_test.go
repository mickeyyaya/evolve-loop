package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestSyncToOriginMovesWhenTheFetchConfigOmitsMain(t *testing.T) {
	t.Parallel()
	origin := gittest.Bare(t)
	work := gittest.Clone(t, origin.Dir)
	work.Write("a.txt", "a\n")
	work.Git("add", ".")
	work.Git("commit", "-q", "-m", "one")
	work.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	work.Git("fetch", "-q", "origin")
	work.Git("config", "remote.origin.fetch", "+refs/heads/other:refs/remotes/origin/other")

	other := gittest.Clone(t, origin.Dir)
	other.Write("b.txt", "b\n")
	other.Git("add", ".")
	other.Git("commit", "-q", "-m", "two")
	other.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	tip := strings.TrimSpace(other.Git("rev-parse", "HEAD"))

	got, err := SyncToOrigin(context.Background(), Repo{Dir: work.Dir, Git: SysGit()})
	if err != nil || got != SyncMoved {
		t.Fatalf("SyncToOrigin = %q, %v, want %q", got, err, SyncMoved)
	}
	if head := strings.TrimSpace(work.Git("rev-parse", "refs/heads/main")); head != tip {
		t.Errorf("local main = %s, want %s", head, tip)
	}
}
