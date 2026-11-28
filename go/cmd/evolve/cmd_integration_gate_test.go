package main

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type gateHarness struct {
	mu        sync.Mutex
	ran       map[string]int
	dirs      []string
	checkouts int
	released  int
	redStep   map[string]bool
	redIfFile string
	runErr    error
	checkErr  error
}

func (h *gateHarness) checkout(_ context.Context, _ string) (string, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkouts++
	if h.checkErr != nil {
		return "", nil, h.checkErr
	}
	return "/tree", func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.released++
	}, nil
}

func (h *gateHarness) run(_ context.Context, dir string, argv ...string) (string, int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ran == nil {
		h.ran = map[string]int{}
	}
	h.ran[strings.Join(argv, " ")]++
	h.dirs = append(h.dirs, dir)
	if h.runErr != nil {
		return "", 0, h.runErr
	}
	if h.redIfFile != "" && argv[1] == "build" {
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), h.redIfFile)); err == nil {
			return "build is red\n", 1, nil
		}
	}
	if h.redStep[argv[1]] {
		return argv[1] + " is red\n", 1, nil
	}
	return "", 0, nil
}

func TestL2Gate_AGreenTreeReturnsNilAndRunsTheStepsOfTheTierInTheGoDirectory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tier overlap.Tier
		want map[string]int
	}{
		{overlap.T1, map[string]int{"go build ./...": 1}},
		{overlap.T2, map[string]int{"go build ./...": 1}},
		{overlap.T3, map[string]int{"go build ./...": 1, "go vet ./...": 1}},
		{overlap.T4, map[string]int{"go build ./...": 1, "go vet ./...": 1}},
	}
	for _, tc := range cases {
		h := &gateHarness{}
		if err := l2Gate(h.checkout, h.run)(context.Background(), "tip", tc.tier); err != nil {
			t.Fatalf("tier %s: green gate returned %v", tc.tier, err)
		}
		if !maps.Equal(h.ran, tc.want) {
			t.Errorf("tier %s ran %v, want %v", tc.tier, h.ran, tc.want)
		}
		for _, dir := range h.dirs {
			if dir != filepath.Join("/tree", "go") {
				t.Errorf("tier %s ran in %q, want the go directory of the checkout", tc.tier, dir)
			}
		}
		if h.checkouts != 1 || h.released != 1 {
			t.Errorf("tier %s: %d checkouts, %d releases, want 1 and 1", tc.tier, h.checkouts, h.released)
		}
	}
}

func TestL2Gate_ARedStepReturnsAGateRedNamingEachRedStepAndStillReleasesTheCheckout(t *testing.T) {
	t.Parallel()
	h := &gateHarness{redStep: map[string]bool{"build": true, "vet": true}}

	err := l2Gate(h.checkout, h.run)(context.Background(), "tip", overlap.T3)

	var red *integration.GateRed
	if !errors.As(err, &red) {
		t.Fatalf("red gate returned %v, want a *GateRed", err)
	}
	if !slices.Equal(red.Failures, []string{"build", "vet"}) {
		t.Errorf("failures = %v, want [build vet]", red.Failures)
	}
	if h.released != 1 {
		t.Errorf("released %d checkouts, want 1", h.released)
	}
}

func TestL2Gate_ARedStepOfALowTierNamesOnlyThatStep(t *testing.T) {
	t.Parallel()
	h := &gateHarness{redStep: map[string]bool{"build": true, "vet": true}}

	err := l2Gate(h.checkout, h.run)(context.Background(), "tip", overlap.T1)

	var red *integration.GateRed
	if !errors.As(err, &red) || !slices.Equal(red.Failures, []string{"build"}) {
		t.Fatalf("T1 red gate returned %v, want a *GateRed naming build only", err)
	}
}

func TestL2Gate_ACommandThatCannotRunIsAHostFailureAndNotAGateRed(t *testing.T) {
	t.Parallel()
	h := &gateHarness{runErr: errors.New("go: not found")}

	err := l2Gate(h.checkout, h.run)(context.Background(), "tip", overlap.T3)

	var red *integration.GateRed
	if err == nil || errors.As(err, &red) || !acsrunner.IsHostFailure(err) {
		t.Fatalf("unrunnable gate returned %v, want a host failure that is not a *GateRed", err)
	}
	if h.released != 1 {
		t.Errorf("released %d checkouts, want 1", h.released)
	}
}

func TestL2Gate_AFailedCheckoutIsAPlainErrorAndRunsNoCommand(t *testing.T) {
	t.Parallel()
	boom := errors.New("worktree add failed")
	h := &gateHarness{checkErr: boom}

	err := l2Gate(h.checkout, h.run)(context.Background(), "tip", overlap.T3)

	var red *integration.GateRed
	if !errors.Is(err, boom) || errors.As(err, &red) {
		t.Fatalf("failed checkout returned %v, want the checkout error and no *GateRed", err)
	}
	if len(h.ran) != 0 {
		t.Errorf("ran %v after a failed checkout", h.ran)
	}
}

func TestIntegrationTrain_TheL2ComposerGateIsTheRealGate(t *testing.T) {
	t.Parallel()
	root := gittest.Fixture(t)
	root.Write("seed.txt", "base\n")
	root.Git("add", "-A")
	root.Git("commit", "-q", "-m", "seed")
	tip := root.Git("rev-parse", "HEAD")
	writeL2Policy(t, root.Dir, enforceL2PolicyJSON)
	env := productionIntegrationEnv(root.Dir, time.Date(2026, 11, 26, 5, 41, 12, 0, time.UTC))
	h := &gateHarness{redStep: map[string]bool{"build": true}}
	env.Run = h.run

	gate := trainOf(env, root.Dir).L2.Composer.Gate

	if gate == nil {
		t.Fatal("the L2 composer has no gate")
	}
	var red *integration.GateRed
	if err := gate(context.Background(), tip, overlap.T3); !errors.As(err, &red) || !slices.Equal(red.Failures, []string{"build"}) {
		t.Fatalf("the L2 gate returned %v on a red build, want a *GateRed naming build", err)
	}
	if want := map[string]int{"go build ./...": 1, "go vet ./...": 1}; !maps.Equal(h.ran, want) {
		t.Errorf("the L2 gate ran %v, want %v", h.ran, want)
	}
}

type gateWorld struct {
	origin *gittest.Repo
	plane  *gittest.Repo
	base   string
	repo   integration.Repo
}

func newGateWorld(t *testing.T) *gateWorld {
	t.Helper()
	seed := gittest.Fixture(t)
	seed.Write("seed.txt", "base\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")
	plane := gittest.Clone(t, origin.Dir)
	return &gateWorld{
		origin: origin,
		plane:  plane,
		base:   seed.Git("rev-parse", "HEAD"),
		repo:   integration.Repo{Dir: plane.Dir, Git: integration.SysGit()},
	}
}

func (w *gateWorld) deliver(t *testing.T, cycle int, file string) integration.DeliverableRecord {
	t.Helper()
	lane := gittest.Clone(t, w.origin.Dir)
	lane.Write(file, "from cycle\n"+file+"\n")
	lane.Git("add", "-A")
	lane.Git("commit", "-q", "-m", "lane "+file)
	sha := lane.Git("rev-parse", "HEAD")
	w.plane.Git("fetch", "-q", lane.Dir, sha)
	patch, err := w.repo.PatchID(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	rec := integration.DeliverableRecord{
		Wave: "7", Cycle: cycle, Trailer: integration.CycleTrailer(cycle),
		Commits:     []integration.CommitIdentity{{SHA: sha, BaseSHA: w.base, PatchID: patch}},
		Audit:       integration.AuditBinding{TreeSHA: strings.Repeat("1", 40), ArtifactSHA: strings.Repeat("a", 64)},
		Scope:       []string{file},
		Risk:        "low",
		DeliveredAt: time.Now().UTC(),
	}
	if _, err := integration.WriteRecord(integration.WaveDir(w.plane.Dir, "7"), rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

type gateLedger struct {
	mu       sync.Mutex
	requeued []int
	promoted []int
	briefs   []*integration.Ejection
}

func (g *gateLedger) Requeue(_ context.Context, rec integration.DeliverableRecord, _ string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requeued = append(g.requeued, rec.Cycle)
	return nil
}

func (g *gateLedger) Promote(_ context.Context, rec integration.DeliverableRecord) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.promoted = append(g.promoted, rec.Cycle)
	return nil
}

func (g *gateLedger) Write(_ context.Context, ej *integration.Ejection) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.briefs = append(g.briefs, ej)
	return "", nil
}

type gateTrain struct {
	w      *gateWorld
	ledger *gateLedger
	cmds   *gateHarness
	driver *integration.Driver
}

func newGateTrain(t *testing.T, files ...string) *gateTrain {
	t.Helper()
	w := newGateWorld(t)
	for i, file := range files {
		w.deliver(t, 11+i, file)
	}
	evolveDir := filepath.Join(w.plane.Dir, ".evolve")
	writeIntegrationStage(t, evolveDir, policy.FleetIntegrationEnforce)
	cmds := &gateHarness{redIfFile: "bad.txt"}
	ledger := &gateLedger{}
	env := productionIntegrationEnv(w.plane.Dir, time.Now())
	env.Run = cmds.run
	b, _ := driverCoordinator(t)
	b.cfg = loopConfig{ProjectRoot: w.plane.Dir, EvolveDir: evolveDir, Epoch: integration.NewEpochHolder(evolveDir)}
	seedCleanShadowPools(t, b, w.plane.Dir, 1, 2)
	b.integrationEnvOf = func(string, time.Time) integrationEnv { return env }
	b.newEnforceActor = func(r integration.Repo, pool int) integration.Actor {
		actor, ok := b.productionEnforceActor(r, pool).(integration.EnforceActor)
		if !ok {
			t.Fatal("productionEnforceActor does not return an EnforceActor")
		}
		actor.Merger = integration.Merger{CI: greenCI{}, Main: integration.GitMain{Repo: r}}
		actor.Requeue, actor.Promote, actor.Briefs = ledger, ledger, ledger
		return actor
	}
	d, err := b.productionDriver(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Check(); err != nil {
		t.Fatalf("Check with the production batcher and the three injected ports = %v, want no missing port", err)
	}
	return &gateTrain{w: w, ledger: ledger, cmds: cmds, driver: d}
}

func (g *gateTrain) endWave(t *testing.T) {
	t.Helper()
	if err := g.driver.EndWave(context.Background()); err != nil {
		t.Fatalf("EndWave: %v", err)
	}
}

func TestProductionDriver_TheProductionBatcherNamesNoMissingPort(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	b.integrationEnvOf = func(string, time.Time) integrationEnv { return productionIntegrationEnv(b.cfg.ProjectRoot, time.Now()) }
	actor, ok := b.productionEnforceActor(integration.Repo{Dir: b.cfg.ProjectRoot, Git: integration.SysGit()}, 7).(integration.EnforceActor)
	if !ok {
		t.Fatal("productionEnforceActor does not return an EnforceActor")
	}
	if missing := actor.Batcher.Missing(); len(missing) != 0 {
		t.Fatalf("the production batcher names %v as missing, want none", missing)
	}
}

func TestGateJourney_AGreenBatchMergesEveryMemberAndRunsTheGateOnEach(t *testing.T) {
	t.Parallel()
	g := newGateTrain(t, "a.txt", "b.txt")

	g.endWave(t)

	files := strings.Fields(g.w.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"))
	if !slices.Contains(files, "a.txt") || !slices.Contains(files, "b.txt") {
		t.Fatalf("origin main holds %v, want both members", files)
	}
	if len(g.ledger.requeued) != 0 {
		t.Errorf("requeued %v on a green batch, want none", g.ledger.requeued)
	}
	if want := map[string]int{"go build ./...": 2, "go vet ./...": 2}; !maps.Equal(g.cmds.ran, want) {
		t.Errorf("the gate ran %v, want build and vet once for each member: %v", g.cmds.ran, want)
	}
}

func TestGateJourney_ARedBatchMemberEjectsThroughTheBatchSplitAndTheGreenMemberMerges(t *testing.T) {
	t.Parallel()
	g := newGateTrain(t, "a.txt", "bad.txt")

	g.endWave(t)

	files := strings.Fields(g.w.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"))
	if !slices.Contains(files, "a.txt") || slices.Contains(files, "bad.txt") {
		t.Fatalf("origin main holds %v, want the green member and not the red one", files)
	}
	if !slices.Equal(g.ledger.requeued, []int{12}) {
		t.Errorf("requeued %v, want only cycle 12", g.ledger.requeued)
	}
	if len(g.ledger.briefs) != 1 || g.ledger.briefs[0].Cause != integration.CauseGate || g.ledger.briefs[0].Cycle != 12 {
		t.Fatalf("briefs = %+v, want one gate ejection of cycle 12", g.ledger.briefs)
	}
	var red *integration.GateRed
	if !errors.As(g.ledger.briefs[0].Err, &red) || !slices.Contains(red.Failures, "build") {
		t.Errorf("the ejection error %v is not a *GateRed naming the build step", g.ledger.briefs[0].Err)
	}
}

func TestGateSoak_ABatchOfMembersCostsTwoGateCommandsPerMemberAndMergesAll(t *testing.T) {
	t.Parallel()
	files := []string{"a.txt", "b.txt", "c.txt", "d.txt"}
	g := newGateTrain(t, files...)

	started := time.Now()
	g.endWave(t)
	elapsed := time.Since(started)

	want := map[string]int{"go build ./...": len(files), "go vet ./...": len(files)}
	if !maps.Equal(g.cmds.ran, want) {
		t.Errorf("the gate ran %v for %d members, want %v", g.cmds.ran, len(files), want)
	}
	merged := strings.Fields(g.w.origin.Git("ls-tree", "-r", "--name-only", "refs/heads/main"))
	for _, f := range files {
		if !slices.Contains(merged, f) {
			t.Errorf("origin main lacks %s after the soak batch", f)
		}
	}
	t.Logf("BatchTest cost per batch: %s for %d members", elapsed, len(files))
}
