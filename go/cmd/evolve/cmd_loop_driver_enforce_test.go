package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type greenCI struct{}

func (greenCI) Open(_ context.Context, b integration.Batch) (integration.PR, error) {
	return integration.PR{Number: 77, Tip: b.Tip}, nil
}
func (greenCI) Watch(context.Context, integration.PR) (integration.Verdict, error) {
	return integration.Green, nil
}
func (greenCI) Rerun(context.Context, integration.PR) error { return nil }

type okPorts struct{}

func (okPorts) Requeue(context.Context, integration.DeliverableRecord, string) error { return nil }
func (okPorts) Promote(context.Context, integration.DeliverableRecord) error         { return nil }
func (okPorts) Write(context.Context, *integration.Ejection) (string, error)         { return "", nil }

func writeIntegrationStage(t *testing.T, evolveDir, stage string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeE2File(t, filepath.Join(evolveDir, "policy.json"), `{"fleet":{"integration":{"stage":"`+stage+`"}}}`)
}

func TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	b.cfg.Epoch = integration.NewEpochHolder(b.cfg.EvolveDir)
	writeIntegrationStage(t, b.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	d, err := b.productionDriver(7)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.Check(); err != nil {
		t.Fatalf("Check at enforce = %v, want nil with every port wired", err)
	}
}

func TestProductionDriver_AtShadowTheDriverStartsAndBuildsNoMerge(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	b.cfg.Epoch = integration.NewEpochHolder(b.cfg.EvolveDir)
	d, err := b.productionDriver(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Check(); err != nil {
		t.Fatalf("Check at shadow = %v", err)
	}
	shadow, ok := d.Actors[policy.FleetIntegrationShadow].(integration.ShadowActor)
	if !ok || len(shadow.Records().Listed) != 0 || shadow.Refresh(context.Background()) == nil {
		t.Fatalf("shadow actor = %+v, ok %v; want no records and a refresh that fails with no origin", shadow, ok)
	}
}

func TestProductionDriver_AtEnforceADeliveredLaneRecordReachesMainAndThePlaneEqualsOrigin(t *testing.T) {
	t.Parallel()
	seed := gittest.Fixture(t)
	seed.Write("a.txt", "base\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	base := seed.Git("rev-parse", "HEAD")
	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")
	plane := gittest.Clone(t, origin.Dir)
	lane := gittest.Clone(t, origin.Dir)
	lane.Write("a.txt", "from the lane\n")
	lane.Git("add", "-A")
	lane.Git("commit", "-q", "-m", "lane")
	sha := lane.Git("rev-parse", "HEAD")
	plane.Git("fetch", "-q", lane.Dir, sha)

	repo := integration.Repo{Dir: plane.Dir, Git: integration.SysGit()}
	patch, err := repo.PatchID(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	evolveDir := filepath.Join(plane.Dir, ".evolve")
	writeIntegrationStage(t, evolveDir, policy.FleetIntegrationEnforce)
	rec := integration.DeliverableRecord{
		Wave: "7", Cycle: 11, Trailer: integration.CycleTrailer(11),
		Commits:     []integration.CommitIdentity{{SHA: sha, BaseSHA: base, PatchID: patch}},
		Audit:       integration.AuditBinding{TreeSHA: strings.Repeat("1", 40), ArtifactSHA: strings.Repeat("a", 64)},
		Scope:       []string{"a.txt"},
		Risk:        "low",
		DeliveredAt: time.Now().UTC(),
	}
	if _, err := integration.WriteRecord(integration.WaveDir(plane.Dir, "7"), rec); err != nil {
		t.Fatal(err)
	}

	b, _ := driverCoordinator(t)
	b.cfg = loopConfig{ProjectRoot: plane.Dir, EvolveDir: evolveDir, Epoch: integration.NewEpochHolder(evolveDir)}
	b.newEnforceActor = func(r integration.Repo, _ int) integration.Actor {
		return integration.EnforceActor{
			Repo:    r,
			Merger:  integration.Merger{CI: greenCI{}, Main: integration.GitMain{Repo: r}},
			Batcher: integration.LaneBatcher{Repo: r, Composer: integration.Composer{Repo: r, Gate: func(context.Context, string, overlap.Tier) error { return nil }}, Wave: "p7"},
			Plane:   integration.PlaneOf(r),
			Requeue: okPorts{}, Promote: okPorts{}, Briefs: okPorts{},
		}
	}
	d, err := b.productionDriver(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Check(); err != nil {
		t.Fatalf("Check with every port injected = %v", err)
	}

	if err := d.EndWave(context.Background()); err != nil {
		t.Fatalf("EndWave: %v", err)
	}

	if got, want := plane.Git("rev-parse", "main"), origin.Git("rev-parse", "main"); got != want || got == base {
		t.Fatalf("plane main %s, origin main %s, base %s; want equal and moved", got, want, base)
	}
	if got := origin.Git("show", "main:a.txt"); got != "from the lane" {
		t.Fatalf("a.txt on origin = %q", got)
	}
	stored, found, err := (integration.FileWaveStore{Root: plane.Dir}).Read(7)
	if err != nil || !found || stored.Members[11].Status != integration.MemberMerged || !stored.Members[11].Landed {
		t.Fatalf("wave record = %+v, found %v, err %v; want member 11 merged and landed", stored.Members, found, err)
	}
	if len(stored.Batches) != 1 || stored.Batches[0].PR != 77 || stored.Batches[0].Base != base {
		t.Fatalf("batches = %+v, want one batch cut at the base with PR 77", stored.Batches)
	}
}
