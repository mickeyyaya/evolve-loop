//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	journeyLaneEnv        = "JOURNEY_LANE_MODE"
	journeyLaneShip       = "ship"
	journeyLaneNoRecord   = "ship-without-record"
	journeyLaneHalt       = "ship-then-halt"
	journeyArtifactDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	journeyEpoch          = 1_800_000_000
)

func TestJourneyLaneHelper(t *testing.T) {
	t.Parallel()
	mode := os.Getenv(journeyLaneEnv)
	if mode == "" {
		return
	}
	os.Exit(runJourneyLane(mode, laneArgsAfterSeparator(os.Args)))
}

func laneArgsAfterSeparator(argv []string) []string {
	for i, a := range argv {
		if a == "--" {
			return argv[i+1:]
		}
	}
	return nil
}

func laneFlag(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func runJourneyLane(mode string, args []string) int {
	root := laneFlag(args, "--project-root")
	pool, err := strconv.Atoi(laneFlag(args, "--pool"))
	if err != nil || pool < 1 || root == "" {
		fmt.Fprintf(os.Stderr, "journey lane: pool %q root %q\n", laneFlag(args, "--pool"), root)
		return 9
	}
	if err := journeyShip(mode, root, pool); err != nil {
		fmt.Fprintf(os.Stderr, "journey lane: %v\n", err)
		return 8
	}
	if mode == journeyLaneHalt {
		return systemFailureHaltExitCode
	}
	return 0
}

func journeyShip(mode, root string, pool int) error {
	ctx := context.Background()
	evolveDir := filepath.Join(root, ".evolve")
	cycle, err := core.AllocateCycleNumber(ctx, storage.New(evolveDir))
	if err != nil {
		return err
	}
	repo := integration.Repo{Dir: root, Git: integration.SysGit()}
	commit, err := journeyCommit(ctx, repo, cycle)
	if err != nil {
		return err
	}
	intent := landing.Intent{Cycle: cycle, CommitSHA: commit.SHA, Status: landing.IntentComplete}
	if err := landing.WriteIntent(landing.IntentPath(evolveDir, cycle), intent); err != nil {
		return err
	}
	if mode == journeyLaneNoRecord {
		return nil
	}
	tree, err := repo.Git(ctx, root, "", "rev-parse", commit.SHA+"^{tree}")
	if err != nil {
		return err
	}
	wave := strconv.Itoa(pool)
	_, err = integration.WriteRecord(integration.WaveDir(root, wave), integration.DeliverableRecord{
		Wave:        wave,
		Cycle:       cycle,
		Trailer:     integration.CycleTrailer(cycle),
		Commits:     []integration.CommitIdentity{commit},
		Audit:       integration.AuditBinding{TreeSHA: strings.TrimSpace(tree), ArtifactSHA: journeyArtifactDigest},
		DeliveredAt: time.Unix(journeyEpoch+int64(cycle), 0).UTC(),
	})
	return err
}

func journeyCommit(ctx context.Context, repo integration.Repo, cycle int) (integration.CommitIdentity, error) {
	git := func(stdin string, args ...string) (string, error) {
		out, err := repo.Git(ctx, repo.Dir, stdin, args...)
		return strings.TrimSpace(out), err
	}
	base, err := git("", "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	blob, err := git(fmt.Sprintf("lane %d\n", cycle), "hash-object", "-w", "--stdin")
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	listing, err := repo.Git(ctx, repo.Dir, "", "ls-tree", base+"^{tree}")
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	tree, err := git(listing+fmt.Sprintf("100644 blob %s\tlane-%d.txt\n", blob, cycle), "mktree")
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	sha, err := git("", "-c", "user.name=lane", "-c", "user.email=lane@example.com", "commit-tree", tree, "-p", base, "-m", fmt.Sprintf("evolve-cycle %d: lane work\n\n%s: %d", cycle, evidence.KeyCycle, cycle))
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	patch, err := repo.PatchID(ctx, sha)
	if err != nil {
		return integration.CommitIdentity{}, err
	}
	return integration.CommitIdentity{SHA: sha, BaseSHA: base, PatchID: patch}, nil
}

type journeyFixture struct {
	e2WindowFixture
	origin *gittest.Repo
	repo   *gittest.Repo
	bin    string
}

func newJourneyFixture(t *testing.T, mode string) journeyFixture {
	t.Helper()
	origin := gittest.Bare(t)
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	repo.Git("fetch", "-q", "origin")
	evolveDir := filepath.Join(repo.Dir, ".evolve")
	for name, body := range e2TwoItems {
		writeE2File(t, filepath.Join(evolveDir, "inbox", name), body)
	}
	st := storage.New(evolveDir)
	b, console := iscsCoordinator(repo.Dir, evolveDir, st)
	b.cfg.GoalHash = "journey"
	b.cfg.Epoch = integration.NewEpochHolder(evolveDir)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "evolve")
	fakeclitest.Install(t, bin, fmt.Sprintf("#!/bin/sh\nexport %s=%s\nexec '%s' -test.run='^TestJourneyLaneHelper$' -test.count=1 -- \"$@\"\n", journeyLaneEnv, mode, self))
	return journeyFixture{e2WindowFixture: e2WindowFixture{root: repo.Dir, evolveDir: evolveDir, b: b, console: console}, origin: origin, repo: repo, bin: bin}
}

type journeyStatus struct {
	Pool    int            `json:"pool"`
	Verdict string         `json:"verdict"`
	Failed  []string       `json:"failed"`
	Counts  map[string]int `json:"counts"`
	Ended   bool           `json:"ended"`
	Base    string         `json:"shadow_base"`
}

func (f journeyFixture) status(t *testing.T, args ...string) (journeyStatus, int) {
	t.Helper()
	verb := integrationVerbs["status"]
	if verb == nil {
		t.Fatal("evolve integration status is not a verb")
	}
	var stdout, stderr bytes.Buffer
	env := integrationEnv{Git: integration.SysGit(), Root: f.root}
	code := verb(env, append(args, "--json"), &stdout, &stderr)
	var got journeyStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("status output is not JSON (exit %d): %v\nstdout:\n%s\nstderr:\n%s", code, err, stdout.String(), stderr.String())
	}
	return got, code
}

func (f journeyFixture) refs(t *testing.T, r *gittest.Repo, patterns ...string) string {
	t.Helper()
	return r.Git(append([]string{"for-each-ref"}, patterns...)...)
}

type journeyRoute struct {
	name  string
	fleet policy.FleetConfig
	bench bool
	lanes int
}

var journeyRoutes = []journeyRoute{
	{name: "pool", fleet: e2PoolFleet, lanes: 2},
	{name: "wave", fleet: policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}, lanes: 2},
	{name: "repair", fleet: policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}, bench: true, lanes: loopwave.RepairWidth},
}

func (f journeyFixture) dispatch(t *testing.T, r journeyRoute) {
	t.Helper()
	if r.bench {
		entry := clihealth.Entry{Family: "claude", Reason: "rate_limit", BenchedAt: time.Now(), BenchedUntil: time.Now().Add(time.Hour)}
		if err := clihealth.NewStore(f.root, nil).Bench(entry); err != nil {
			t.Fatal(err)
		}
	}
	f.b.dispatchFleetIteration(0, r.fleet, f.bin, &fleet.StarvationTracker{})
	f.b.deps.Signals.Flush()
	if r.bench && !strings.Contains(f.console.String(), "min-width repair dispatched") {
		t.Fatalf("the repair route did not run:\n%s", f.console.String())
	}
}

func TestJourney_EachRouteEndsInACleanWaveThatStatusReads(t *testing.T) {
	t.Parallel()
	for _, r := range journeyRoutes {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			f := newJourneyFixture(t, journeyLaneShip)
			originBefore, trackingBefore := f.refs(t, f.origin), f.refs(t, f.repo, "refs/remotes/origin")

			f.dispatch(t, r)
			got, code := f.status(t)

			if got.Pool < 1 {
				t.Fatalf("status names pool %d, want a minted pool above 0\nconsole:\n%s", got.Pool, f.console.String())
			}
			if got.Verdict != "clean" || code != 0 || !got.Ended {
				t.Fatalf("verdict %q exit %d ended=%v failed=%v counts=%v\nconsole:\n%s", got.Verdict, code, got.Ended, got.Failed, got.Counts, f.console.String())
			}
			if got.Counts["composed"] != r.lanes || got.Counts["shipped"] != r.lanes {
				t.Errorf("composed = %d shipped = %d, want %d of each (counts %v)", got.Counts["composed"], got.Counts["shipped"], r.lanes, got.Counts)
			}
			if after := f.refs(t, f.origin); after != originBefore {
				t.Errorf("shadow pushed to origin:\nbefore %s\nafter  %s", originBefore, after)
			}
			if after := f.refs(t, f.repo, "refs/remotes/origin"); after != trackingBefore {
				t.Errorf("shadow moved refs/remotes/origin:\nbefore %s\nafter  %s", trackingBefore, after)
			}
			if left := f.refs(t, f.repo, "refs/heads/integration"); left != "" {
				t.Errorf("wave end left local integration refs:\n%s", left)
			}
		})
	}
}

func TestJourney_AShippedLaneWithNoRecordReadsNotClean(t *testing.T) {
	t.Parallel()
	f := newJourneyFixture(t, journeyLaneNoRecord)

	f.dispatch(t, journeyRoutes[0])
	got, code := f.status(t)

	if got.Pool < 1 || got.Verdict != "not clean" || code == 0 {
		t.Fatalf("pool %d verdict %q exit %d, want pool above 0, not clean and a failing exit\nconsole:\n%s", got.Pool, got.Verdict, code, f.console.String())
	}
	if !strings.Contains(strings.Join(got.Failed, "\n"), "missing record") {
		t.Errorf("failed = %v, want the missing record named", got.Failed)
	}
}

func TestJourney_TheHeldBaseIsTheLiveOriginMainAndTheTrackingRefStaysWhereItWas(t *testing.T) {
	t.Parallel()
	f := newJourneyFixture(t, journeyLaneShip)
	other := gittest.Clone(t, f.origin.Dir)
	other.Git("commit", "-q", "--allow-empty", "-m", "a console merge")
	other.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	moved := strings.TrimSpace(other.Git("rev-parse", "HEAD"))
	tracking := f.refs(t, f.repo, "refs/remotes/origin")

	f.dispatch(t, journeyRoutes[0])
	got, _ := f.status(t)

	if got.Base != moved {
		t.Errorf("shadow base = %s, want the live origin main %s", got.Base, moved)
	}
	if after := f.refs(t, f.repo, "refs/remotes/origin"); after != tracking {
		t.Errorf("the held-base fetch moved the tracking ref:\nbefore %s\nafter  %s", tracking, after)
	}
	if got.Verdict != "clean" {
		t.Errorf("verdict %q failed=%v: a lane forked before the console merge is not a conflict", got.Verdict, got.Failed)
	}
}

type endedProbe struct {
	inner io.Writer
	root  string
	ended []bool
}

func (p *endedProbe) Write(b []byte) (int, error) {
	if strings.Contains(string(b), "system_failure_halt") {
		store := integration.FileWaveStore{Root: p.root}
		pools := store.Pools()
		rec, found, _ := store.Read(pools[len(pools)-1])
		p.ended = append(p.ended, found && rec.Ended)
	}
	return p.inner.Write(b)
}

func TestJourney_TheWaveRecordEndsBeforeTheHaltWritesItsFinalJSON(t *testing.T) {
	t.Parallel()
	for _, r := range journeyRoutes[:2] {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			f := newJourneyFixture(t, journeyLaneHalt)
			probe := &endedProbe{inner: f.b.stdout, root: f.root}
			f.b.stdout = probe

			d := f.b.dispatchFleetIteration(0, r.fleet, f.bin, &fleet.StarvationTracker{})
			f.b.deps.Signals.Flush()

			if d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode {
				t.Fatalf("decision = %+v, want the halt\nconsole:\n%s", d, f.console.String())
			}
			if len(probe.ended) != 1 || !probe.ended[0] {
				t.Fatalf("Ended at the final JSON = %v, want one write and the record already ended", probe.ended)
			}
		})
	}
}
