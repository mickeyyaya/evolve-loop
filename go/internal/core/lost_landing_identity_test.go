package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const identityCycle = 2147

type rewrittenLanding struct {
	repo, base, lane, laneTree string
	record                     integration.DeliverableRecord
}

func composedLanding(t *testing.T, trailer string) rewrittenLanding {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("base.txt", "base "+r.Dir+"\n")
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "base")
	base := gitRev(t, r.Dir, "HEAD")
	r.Git("checkout", "-q", "-b", "lane")
	r.Write("delivered.txt", "the lane work "+r.Dir+"\n")
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "the lane work")
	lane := gitRev(t, r.Dir, "HEAD")
	patch, err := (integration.Repo{Dir: r.Dir, Git: integration.SysGit()}).PatchID(context.Background(), lane)
	if err != nil {
		t.Fatal(err)
	}
	r.Git("checkout", "-q", "-")
	r.Git("cherry-pick", "--no-commit", lane)
	r.Git("commit", "-q", "-m", "compose: the lane work\n\n"+trailer)
	rec := integration.DeliverableRecord{Wave: "w1", Cycle: identityCycle, Trailer: integration.CycleTrailer(identityCycle),
		Commits: []integration.CommitIdentity{{SHA: lane, BaseSHA: base, PatchID: patch}}}
	if _, err := integration.WriteRecord(filepath.Join(r.Dir, ".evolve", "waves", "integration-w1"), rec); err != nil {
		t.Fatal(err)
	}
	return rewrittenLanding{repo: r.Dir, base: base, lane: lane, laneTree: gitRev(t, r.Dir, lane+"^{tree}"), record: rec}
}

func detectForRewritten(t *testing.T, l rewrittenLanding) *SystemFailureSignal {
	t.Helper()
	ws := shipRanAndFailed(t)
	plantBinding(t, ws, dossier.ShipBinding{CommitSHA: l.lane, TreeSHACommitted: l.laneTree, Cycle: identityCycle})
	return detectLostLanding(landingEvidence{Workspace: ws, ProjectRoot: l.repo, PreCycleHEAD: l.base, Cycle: identityCycle, FinalVerdict: VerdictPASS})
}

func TestDetectLostLanding_ACycleRewrittenByAComposeIsFoundByTrailerAndPatchID(t *testing.T) {
	t.Parallel()
	l := composedLanding(t, integration.CycleTrailer(identityCycle))
	if sig := detectForRewritten(t, l); sig != nil {
		t.Fatalf("the cycle landed under a new SHA with its trailer and patch, so no landing is lost: %+v", sig)
	}
}

func TestDetectLostLanding_ARewrittenCommitWithoutTheTrailerIsStillLost(t *testing.T) {
	t.Parallel()
	l := composedLanding(t, "no trailer here")
	if sig := detectForRewritten(t, l); sig == nil {
		t.Fatal("without the cycle trailer the rewritten commit is not this cycle's, so the landing is lost")
	}
}

func TestDetectLostLanding_ARecordThatDoesNotHoldTheBindingCommitDoesNotVouchForIt(t *testing.T) {
	t.Parallel()
	l := composedLanding(t, integration.CycleTrailer(identityCycle))
	ws := shipRanAndFailed(t)
	plantBinding(t, ws, dossier.ShipBinding{CommitSHA: l.base, TreeSHACommitted: gitRev(t, l.repo, l.base+"^{tree}"), Cycle: identityCycle})
	sig := detectLostLanding(landingEvidence{Workspace: ws, ProjectRoot: l.repo, PreCycleHEAD: l.base, Cycle: identityCycle, FinalVerdict: VerdictPASS})
	if sig == nil {
		t.Fatal("the record of the cycle holds the lane commit, not the binding commit, so it must not mask the lost landing")
	}
}
