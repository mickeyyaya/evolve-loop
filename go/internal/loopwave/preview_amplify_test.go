package loopwave

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
)

func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		out[p] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPreviewAmplify_IsDeterministicAcrossRepeatedCalls(t *testing.T) {
	h := newHarness(t)
	for i, id := range []string{"c1", "a1", "b1", "d1"} {
		writeInbox(t, h, id, 0.5, []string{"pkg/" + id + "/x.go"})
		_ = i
	}
	first := h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})
	for n := 0; n < 5; n++ {
		again := h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("call %d differs: %+v vs %+v", n, first, again)
		}
	}
}

func TestPreviewAmplify_WritesNothingUnderEvolveDir(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	before := treeDigest(t, h.evolveDir)
	h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})
	after := treeDigest(t, h.evolveDir)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("preview changed the evolve tree")
	}
}

func TestPreviewAmplify_ZeroAndNegativeCountDoNotPanic(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	for _, n := range []int{0, -1, 1} {
		plan := h.e.Preview(context.Background(), waveFleet(n), nil, nil, budgethistory.Throughput{}, time.Time{})
		if len(plan.Lanes) > 1 {
			t.Errorf("count %d produced %d lanes", n, len(plan.Lanes))
		}
		if plan.Reason == "" {
			t.Errorf("count %d: empty reason", n)
		}
	}
}

func TestPreviewAmplify_ChainedDependencyIsExcluded(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	writeInbox(t, h, "c1", 0.7, []string{"pkg/c/c.go"}, "b1")
	writeInbox(t, h, "d1", 0.6, []string{"pkg/d/d.go"}, "c1")
	plan := h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})
	placed := slices.Concat(laneIDs(plan)...)
	for _, id := range []string{"c1", "d1"} {
		if slices.Contains(placed, id) {
			t.Errorf("%s with unmet dependency placed in a lane: %v", id, placed)
		}
	}
}

func TestPreviewAmplify_AllItemsSharingOneFileFormOneLane(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"a1", "a2", "a3", "a4"} {
		writeInbox(t, h, id, 0.6, []string{"pkg/shared/s.go"})
	}
	plan := h.e.Preview(context.Background(), waveFleet(4), nil, nil, budgethistory.Throughput{}, time.Time{})
	nonEmpty := 0
	for _, ids := range laneIDs(plan) {
		if len(ids) > 0 {
			nonEmpty++
		}
	}
	if nonEmpty > 1 {
		t.Errorf("file-overlapping items split across %d lanes: %v", nonEmpty, laneIDs(plan))
	}
}

func TestPreviewAmplify_NoItemAppearsInTwoLanes(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"a1", "b1", "c1", "d1", "e1", "f1"} {
		writeInbox(t, h, id, 0.5, []string{"pkg/" + id + "/x.go"})
	}
	plan := h.e.Preview(context.Background(), waveFleet(3), nil, nil, budgethistory.Throughput{}, time.Time{})
	seen := map[string]bool{}
	for _, ids := range laneIDs(plan) {
		for _, id := range ids {
			if seen[id] {
				t.Errorf("%s placed twice", id)
			}
			seen[id] = true
		}
	}
	for _, ex := range plan.Excluded {
		if seen[ex.ID] {
			t.Errorf("%s both placed and excluded", ex.ID)
		}
	}
}
