package swarm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// scriptMerger fails the merge for the named branches (until a resolver flips them).
type scriptMerger struct {
	failBranch map[string]bool
	merged     []string
}

func (m *scriptMerger) Merge(_ context.Context, _, fromBranch string) error {
	if m.failBranch[fromBranch] {
		return ErrMergeConflict
	}
	m.merged = append(m.merged, fromBranch)
	return nil
}

func branchMap(ids ...string) map[string]string {
	out := map[string]string{}
	for _, id := range ids {
		out[id] = "cycle-1-" + id
	}
	return out
}

func TestRunMergeTrain_AllCleanInOrder(t *testing.T) {
	m := &scriptMerger{}
	rep := RunMergeTrain(context.Background(), "cycle-1-integration",
		[]string{"w0", "w1", "w2"}, branchMap("w0", "w1", "w2"), MergeTrainDeps{Merger: m})
	if !rep.AllMerged {
		t.Fatalf("all should merge: %+v", rep.Outcomes)
	}
	if len(m.merged) != 3 || m.merged[0] != "cycle-1-w0" || m.merged[2] != "cycle-1-w2" {
		t.Errorf("merge order wrong: %v", m.merged)
	}
}

func TestRunMergeTrain_AcceptanceGateStops(t *testing.T) {
	m := &scriptMerger{}
	accept := func(_ context.Context, workerID, _ string) error {
		if workerID == "w1" {
			return errors.New("go test failed")
		}
		return nil
	}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1", "w2"}, branchMap("w0", "w1", "w2"),
		MergeTrainDeps{Merger: m, Accept: accept})
	if rep.AllMerged {
		t.Fatal("acceptance failure must stop the train")
	}
	if len(rep.Outcomes) != 2 || rep.Outcomes[1].WorkerID != "w1" || rep.Outcomes[1].Merged {
		t.Errorf("expected w0 ok + w1 failed then stop: %+v", rep.Outcomes)
	}
}

func TestRunMergeTrain_ConflictResolvedOnRetry(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w1": true}}
	resolver := func(_ context.Context, workerID, _ string) error {
		if workerID == "w1" {
			m.failBranch["cycle-1-w1"] = false // "fix" so the retry merges
		}
		return nil
	}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1"}, branchMap("w0", "w1"),
		MergeTrainDeps{Merger: m, Resolver: resolver, MaxRetries: 1})
	if !rep.AllMerged {
		t.Fatalf("conflict should be resolved on retry: %+v", rep.Outcomes)
	}
	if !rep.Outcomes[1].Resolved {
		t.Errorf("w1 should be marked Resolved: %+v", rep.Outcomes[1])
	}
}

func TestRunMergeTrain_ConflictNoResolverFails(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w0": true}}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0"}, branchMap("w0"), MergeTrainDeps{Merger: m})
	if rep.AllMerged || rep.Outcomes[0].Merged {
		t.Errorf("conflict with no resolver must fail: %+v", rep.Outcomes)
	}
}

func TestRunMergeTrain_EmptyOrderNotMerged(t *testing.T) {
	rep := RunMergeTrain(context.Background(), "integ", nil, nil, MergeTrainDeps{Merger: &scriptMerger{}})
	if rep.AllMerged {
		t.Error("empty order must not report AllMerged")
	}
}

func TestSynthesize_ReaderConcatOrdered(t *testing.T) {
	got := Synthesize([]string{"w0", "w1"}, map[string]string{"w0": "finding A", "w1": "finding B"})
	if !strings.Contains(got, "## w0") || !strings.Contains(got, "finding A") ||
		!strings.Contains(got, "## w1") || !strings.Contains(got, "finding B") {
		t.Errorf("synthesis missing parts:\n%s", got)
	}
	if strings.Index(got, "## w0") > strings.Index(got, "## w1") {
		t.Error("synthesis order wrong (w0 should precede w1)")
	}
}
