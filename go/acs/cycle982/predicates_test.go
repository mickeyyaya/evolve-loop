//go:build acs

package cycle982

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestC982_001_LiveShipDriverResolvesCulprit(t *testing.T) {
	cfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()

	lanes := []fleet.LaneCandidate{
		{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}},
		{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}},
		{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}},
	}
	calls := 0
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2")
	}

	landed, ejected := ship.LandPrefixes(cfg, lanes, verify)

	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land via ship.LandPrefixes, got landed=%v", landed)
	}
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI (<=6), not a sweep", calls)
	}

	if l, e := ship.LandPrefixes(cfg, nil, verify); len(l) != 0 || len(e) != 0 {
		t.Errorf("empty lane set => landed=%v ejected=%v, want both empty", l, e)
	}
}

func TestC982_002_ResolveCulpritHasShipCaller(t *testing.T) {
	root := acsassert.RepoRoot(t)
	shipDir := root + "/go/internal/phases/ship"
	stdout, _, code, err := acsassert.SubprocessOutput("grep", "-rl", "--include=*.go", "ResolveCulprit", shipDir)
	if err != nil || code != 0 {
		t.Fatalf("ResolveCulprit has no caller under %s (grep code=%d err=%v) — the composer is still INERT, violating the no-inert-API floor", shipDir, code, err)
	}
	prod := false
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line != "" && !strings.HasSuffix(line, "_test.go") {
			prod = true
			break
		}
	}
	if !prod {
		t.Errorf("ResolveCulprit is referenced only from *_test.go under ship (%q) — a test-only caller leaves the composed production path inert", strings.TrimSpace(stdout))
	}
}

func TestC982_003_ConcurrentEnqueueNoLostWork(t *testing.T) {
	const rounds = 20
	const n = 300
	for r := 0; r < rounds; r++ {
		q := fleet.NewPrefixQueue()
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				q.Enqueue(fleet.LaneCandidate{
					ID:    fmt.Sprintf("L%d", i),
					Tier:  fleet.TierMaybe,
					Files: []string{fmt.Sprintf("go/internal/f%d/f.go", i)},
				})
			}(i)
		}
		wg.Wait()
		if got := len(q.ComposePrefixes()); got != n {
			t.Fatalf("round %d: %d lanes survived %d concurrent Enqueues — lost appends (unsynchronized single-writer)", r, got, n)
		}
	}

	q := fleet.NewPrefixQueue()
	for i := 0; i < 5; i++ {
		q.OnGreen()
	}
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() { defer wg.Done(); q.OnRed() }()
	}
	wg.Wait()
	if got := q.Window(); got < 1 {
		t.Fatalf("AIMD window = %d after concurrent reds — floor of 1 breached (torn read-modify-write)", got)
	}
}

func TestC982_004_PrefixQueueGuardsSharedState(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := root + "/go/internal/fleet/prefixqueue.go"
	if !acsassert.FileExists(t, path) {
		t.Fatalf("prefixqueue.go missing at %s", path)
	}
	if !acsassert.FileMatchesRegex(t, path, `sync\.(Mutex|RWMutex)`) {
		t.Errorf("prefixqueue.go declares no sync.Mutex/RWMutex — shared lanes/window are unguarded")
	}
}

func TestC982_005_NoPoisonedCompositeLands(t *testing.T) {
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "A", Tier: fleet.TierIffy, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "B", Tier: fleet.TierIffy, Files: []string{"go/internal/b/b.go"}})
	poison := func(laneIDs []string) bool {
		return !(contains(laneIDs, "A") && contains(laneIDs, "B"))
	}
	landed, _ := q.ResolveCulprit(poison)
	if !poison(landed) {
		t.Errorf("landed set %v fails full re-verification — a poisoned composite landed (F2: verify(landed) must always hold)", landed)
	}
	if contains(landed, "A") && contains(landed, "B") {
		t.Errorf("both A and B landed (%v) despite their union failing — poisoned composite not trimmed", landed)
	}

	q2 := fleet.NewPrefixQueue()
	q2.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/x/x.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/y/y.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/z/z.go"}})
	mid := func(laneIDs []string) bool { return !contains(laneIDs, "L2") }
	landed2, ejected2 := q2.ResolveCulprit(mid)
	if !contains(landed2, "L1") || !contains(landed2, "L3") || contains(landed2, "L2") {
		t.Errorf("independent-failure path changed: landed=%v, want L1,L3 land and L2 eject", landed2)
	}
	if len(ejected2) != 1 || ejected2[0] != "L2" {
		t.Errorf("independent-failure path changed: ejected=%v, want exactly [L2]", ejected2)
	}
	if !mid(landed2) {
		t.Errorf("independent-failure landed set %v fails re-verification", landed2)
	}

	q3 := fleet.NewPrefixQueue()
	q3.Enqueue(fleet.LaneCandidate{ID: "S", Tier: fleet.TierMaybe, Files: []string{"go/internal/s/s.go"}})
	if l, e := q3.ResolveCulprit(func([]string) bool { return true }); len(l) != 1 || l[0] != "S" || len(e) != 0 {
		t.Errorf("single green lane => landed=%v ejected=%v, want [S] / none", l, e)
	}
	if l, e := fleet.NewPrefixQueue().ResolveCulprit(func([]string) bool { return true }); len(l) != 0 || len(e) != 0 {
		t.Errorf("empty queue => landed=%v ejected=%v, want both empty", l, e)
	}
}
