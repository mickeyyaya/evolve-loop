//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func recordingLaneBinary(t *testing.T) (bin, out string) {
	t.Helper()
	out = filepath.Join(t.TempDir(), "lane-args.txt")
	return fakeCycleBinary(t, "echo \"$*\" >> "+out+"\nexit 0\n"), out
}

func laneArgLines(t *testing.T, out string) []string {
	t.Helper()
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no lane ran: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

func assertEveryLaneGotPool(t *testing.T, out string, pool int) {
	t.Helper()
	want := "--pool " + strconv.Itoa(pool)
	for _, line := range laneArgLines(t, out) {
		if !strings.HasSuffix(line, want) {
			t.Errorf("lane args %q do not end with %q", line, want)
		}
	}
}

func TestDispatchPool_TheMintedPoolReachesEveryLane(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin, out := recordingLaneBinary(t)

	f.b.settledPool(0, e2PoolFleet, f.b.cycleLaunch(bin, 41))

	assertEveryLaneGotPool(t, out, 41)
}

func TestRepairMinWidth_TheMintedPoolReachesTheRepairLane(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin, out := recordingLaneBinary(t)
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}

	f.b.settledRepair(0, fc, policy.FleetConfig{Count: 0, PlanSource: "triage"}, f.b.cycleLaunch(bin, 41), &fleet.StarvationTracker{})

	assertEveryLaneGotPool(t, out, 41)
}

func TestDispatchFleetIteration_TheAllocatedPoolReachesTheLanesOfBothRoutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		fc   policy.FleetConfig
	}{
		{"pool", e2PoolFleet},
		{"wave", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}},
	} {
		f := newE2WindowFixture(t, e2TwoItems)
		st := storage.New(f.evolveDir)
		f.b.deps.Storage = st
		bin, out := recordingLaneBinary(t)

		f.b.dispatchFleetIteration(0, tc.fc, bin, &fleet.StarvationTracker{})

		allocated := lastAllocated(t, st)
		if allocated < 1 {
			t.Fatalf("%s: no pool number was allocated", tc.name)
		}
		assertEveryLaneGotPool(t, out, allocated)
	}
}

func TestDispatchFleetIteration_AShrunkWaveRepairsWithTheMintedPool(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	st := storage.New(f.evolveDir)
	f.b.deps.Storage = st
	f.b.cfg.ProjectRoot = f.root
	f.b.cfg.EvolveDir = f.evolveDir
	if err := clihealth.NewStore(f.root, nil).Bench(clihealth.Entry{Family: "claude", Reason: "rate_limit", BenchedAt: time.Now(), BenchedUntil: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	bin, out := recordingLaneBinary(t)
	f.b.dispatchFleetIteration(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}, bin, &fleet.StarvationTracker{})
	f.b.deps.Signals.Flush()
	if !strings.Contains(f.console.String(), "min-width repair dispatched") {
		t.Fatalf("the repair route did not run:\n%s", f.console.String())
	}
	assertEveryLaneGotPool(t, out, lastAllocated(t, st))
}
