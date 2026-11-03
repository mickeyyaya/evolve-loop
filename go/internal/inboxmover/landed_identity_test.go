package inboxmover

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

func commitIn(r *gittest.Repo, name, msg string) string {
	r.Write(name, name+"\n")
	r.Git("add", name)
	r.Git("commit", "-q", "-m", msg)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

func composedRoot(t *testing.T, composeMessage string) (root, lane string) {
	t.Helper()
	r := gittest.Fixture(t)
	base := commitIn(r, "a.txt", "base")
	r.Git("checkout", "-q", "-b", "lane")
	lane = commitIn(r, "b.txt", "the lane work")
	patch, err := integration.Repo{Dir: r.Dir, Git: integration.SysGit()}.PatchID(context.Background(), lane)
	if err != nil {
		t.Fatal(err)
	}
	r.Git("checkout", "-q", "main")
	r.Git("cherry-pick", "--no-commit", lane)
	r.Git("commit", "-q", "-m", composeMessage)
	rec := integration.DeliverableRecord{Wave: "w1", Cycle: 77, Trailer: integration.CycleTrailer(77),
		Commits: []integration.CommitIdentity{{SHA: lane, BaseSHA: base, PatchID: patch}}}
	if _, err := integration.WriteRecord(integration.WaveDir(r.Dir, "w1"), rec); err != nil {
		t.Fatal(err)
	}
	return r.Dir, lane
}

func TestShaLandedOnMain_AnAncestorLandedWithoutARecord(t *testing.T) {
	r := gittest.Fixture(t)
	sha := commitIn(r, "a.txt", "base")
	landed, err := shaLandedOnMain(r.Dir, sha)
	if err != nil || !landed {
		t.Fatalf("landed=%v err=%v, want the ancestry rule to hold", landed, err)
	}
}

func TestShaLandedOnMain_ACommitNotOnMainAndWithoutARecordIsNotLanded(t *testing.T) {
	r := gittest.Fixture(t)
	commitIn(r, "a.txt", "base")
	r.Git("checkout", "-q", "-b", "lane")
	lane := commitIn(r, "b.txt", "lane only")
	landed, err := shaLandedOnMain(r.Dir, lane)
	if err != nil || landed {
		t.Fatalf("landed=%v err=%v, want not landed", landed, err)
	}
}

func TestShaLandedOnMain_ARewrittenCommitIsFoundByTrailerAndPatchID(t *testing.T) {
	root, lane := composedRoot(t, "compose: the lane work\n\n"+integration.CycleTrailer(77))
	landed, err := shaLandedOnMain(root, lane)
	if err != nil || !landed {
		t.Fatalf("landed=%v err=%v, want landed by identity", landed, err)
	}
}

func TestShaLandedOnMain_ARewrittenCommitWithoutTheTrailerIsNotLanded(t *testing.T) {
	root, lane := composedRoot(t, "compose: the lane work")
	landed, err := shaLandedOnMain(root, lane)
	if err != nil || landed {
		t.Fatalf("landed=%v err=%v, want not landed without the trailer", landed, err)
	}
}
