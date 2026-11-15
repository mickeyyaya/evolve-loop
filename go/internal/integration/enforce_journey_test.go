package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type journeyCI struct {
	verdict Verdict
}

func (c journeyCI) Open(_ context.Context, b Batch) (PR, error) {
	return PR{Number: 101, Tip: b.Tip}, nil
}

func (c journeyCI) Watch(context.Context, PR) (Verdict, error) { return c.verdict, nil }
func (c journeyCI) Rerun(context.Context, PR) error            { return nil }

func TestEnforceJourney_FastForwardsCleanMemberAndRequeuesConflict(t *testing.T) {
	seed := gittest.Fixture(t)
	seed.Write("a.txt", "base\n")
	seed.Write("b.txt", "base\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	base := seed.Git("rev-parse", "HEAD")

	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")

	plane := gittest.Clone(t, origin.Dir)
	laneA := gittest.Clone(t, origin.Dir)
	laneB := gittest.Clone(t, origin.Dir)
	other := gittest.Clone(t, origin.Dir)

	laneA.Write("a.txt", "from lane A\n")
	laneA.Git("add", "-A")
	laneA.Git("commit", "-q", "-m", "lane A")
	shaA := laneA.Git("rev-parse", "HEAD")
	plane.Git("fetch", "-q", laneA.Dir, shaA)

	laneB.Write("b.txt", "from lane B\n")
	laneB.Git("add", "-A")
	laneB.Git("commit", "-q", "-m", "lane B")
	shaB := laneB.Git("rev-parse", "HEAD")
	plane.Git("fetch", "-q", laneB.Dir, shaB)

	other.Write("b.txt", "from console\n")
	other.Git("add", "-A")
	other.Git("commit", "-q", "-m", "console commit")
	other.Git("push", "-q", "origin", "main")
	plane.Git("fetch", "-q", "origin", "main")
	liveHead := plane.Git("rev-parse", "origin/main")

	planeRepo := Repo{Dir: plane.Dir, Git: SysGit()}
	patchA, err := planeRepo.PatchID(context.Background(), shaA)
	if err != nil {
		t.Fatal(err)
	}
	patchB, err := planeRepo.PatchID(context.Background(), shaB)
	if err != nil {
		t.Fatal(err)
	}
	artA := strings.Repeat("a", 64)
	artB := strings.Repeat("b", 64)
	treeA := strings.Repeat("1", 40)
	treeB := strings.Repeat("2", 40)
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	recA := DeliverableRecord{
		Wave: "1", Cycle: 11, Trailer: CycleTrailer(11),
		Commits:     []CommitIdentity{{SHA: shaA, BaseSHA: base, PatchID: patchA}},
		Audit:       AuditBinding{TreeSHA: treeA, ArtifactSHA: artA},
		Scope:       []string{"a.txt"},
		Risk:        "low",
		DeliveredAt: now,
	}
	recB := DeliverableRecord{
		Wave: "1", Cycle: 12, Trailer: CycleTrailer(12),
		Commits:     []CommitIdentity{{SHA: shaB, BaseSHA: base, PatchID: patchB}},
		Audit:       AuditBinding{TreeSHA: treeB, ArtifactSHA: artB},
		Scope:       []string{"b.txt"},
		Risk:        "low",
		DeliveredAt: now,
	}

	ports := &stubEnforcePorts{briefPath: filepath.Join(t.TempDir(), "brief-12.txt")}
	comp := Composer{Repo: planeRepo, Stage: policy.FleetIntegrationShadow}
	actor := EnforceActor{
		Repo: planeRepo,
		Merger: Merger{
			CI:   journeyCI{verdict: Green},
			Main: GitMain{Repo: planeRepo},
			Compose: func(ctx context.Context, recs []DeliverableRecord) (Composition, error) {
				b, err := planeRepo.StartBatch(ctx, Batch{Wave: "1", Pool: 1, Index: 1, Base: liveHead})
				if err != nil {
					return Composition{}, err
				}
				var kept []DeliverableRecord
				var ejected []*Ejection
				for _, r := range recs {
					st, cerr := comp.Compose(ctx, b, r)
					var ej *Ejection
					if errors.As(cerr, &ej) {
						ejected = append(ejected, ej)
						continue
					}
					if cerr != nil {
						return Composition{}, cerr
					}
					b = st.Batch
					kept = append(kept, r)
				}
				return Composition{Batch: b, Kept: kept, Ejected: ejected}, nil
			},
		},
		Plane:   planeSyncFn(func(ctx context.Context) (SyncOutcome, error) { return SyncToOrigin(ctx, planeRepo) }),
		Requeue: ports,
		Briefs:  ports,
		Promote: ports,
	}

	res, err := actor.Flush(context.Background(), PendingBatch{
		Pool: 1, Index: 1, Base: liveHead, Members: []DeliverableRecord{recA, recB},
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if res.Members[11].Status != MemberMerged || !res.Members[11].Landed {
		t.Fatalf("member 11 = %+v, want merged and landed", res.Members[11])
	}
	if res.Members[12].Status != MemberEjected || res.Members[12].Landed {
		t.Fatalf("member 12 = %+v, want ejected and not landed", res.Members[12])
	}
	if !slices.Equal(ports.promoted, []int{11}) || !slices.Equal(ports.requeued, []int{12}) {
		t.Fatalf("promoted=%v requeued=%v, want [11] and [12]", ports.promoted, ports.requeued)
	}

	endRec, err := actor.WaveEnd(context.Background(), WaveRecord{Pool: 1, Epoch: 1, ShadowBase: liveHead})
	if err != nil || len(endRec.Failures) != 0 {
		t.Fatalf("WaveEnd: err=%v failures=%v", err, endRec.Failures)
	}
	if gotPlane, gotOrigin := plane.Git("rev-parse", "main"), origin.Git("rev-parse", "main"); gotPlane != gotOrigin {
		t.Fatalf("plane main %s != origin main %s", gotPlane, gotOrigin)
	}
	if raw, err := os.ReadFile(filepath.Join(plane.Dir, "a.txt")); err != nil || string(raw) != "from lane A\n" {
		t.Fatalf("plane a.txt = %q, %v", raw, err)
	}
}

type planeSyncFn func(ctx context.Context) (SyncOutcome, error)

func (f planeSyncFn) Sync(ctx context.Context) (SyncOutcome, error) { return f(ctx) }
