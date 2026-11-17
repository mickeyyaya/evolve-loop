package triagecap

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
)

type Preemption struct {
	Old      string
	New      string
	OldScore float64
	NewScore float64
}

func (p Preemption) Line() string {
	return fmt.Sprintf("preempted %s for %s (score %.4f to %.4f)", p.Old, p.New, p.OldScore, p.NewScore)
}

func PreemptLowestSlot(evolveDir string, committed []FleetCandidate, started map[string]bool, isProtected, allows func(string) bool, now time.Time) ([]FleetCandidate, *Preemption) {
	if len(committed) == 0 {
		return committed, nil
	}
	rank, _ := rankinputs.Load(evolveDir, now)
	lifecycle := readOnlyLifecycle(evolveDir)
	scan, err := inboxbatch.ScanDir(lifecycle.InboxDir)
	if err != nil {
		return committed, nil
	}
	scores := map[string]float64{}
	for _, r := range rank.Order(scan.Items, scan.Items) {
		scores[r.Item.ID] = r.Breakdown.Score
	}
	continuations := map[string]bool{}
	for _, it := range scan.Items {
		continuations[it.ID] = it.Continuation != nil
	}
	victim := lowestUnstartedSlot(committed, scores, continuations, started)
	if victim < 0 {
		return committed, nil
	}
	margin := rank.Config.PreemptMargin
	inPlan := map[string]bool{}
	for _, c := range committed {
		inPlan[c.ID] = true
	}
	for _, r := range inboxmover.RankLaneMenu(lifecycle, scan.Items, isProtected, rank).Ranked {
		lead := r.Breakdown.Score - scores[committed[victim].ID]
		if lead <= 0 || lead < margin {
			break
		}
		if inPlan[r.Item.ID] || !allows(r.Item.ID) || overlapsKept(r.Item.Files, committed, victim) {
			continue
		}
		out := append([]FleetCandidate(nil), committed...)
		out[victim] = FleetCandidate{ID: r.Item.ID, Weight: r.Item.Weight, Files: r.Item.Files}
		return out, &Preemption{Old: committed[victim].ID, New: r.Item.ID, OldScore: scores[committed[victim].ID], NewScore: r.Breakdown.Score}
	}
	return committed, nil
}

func lowestUnstartedSlot(committed []FleetCandidate, scores map[string]float64, continuations, started map[string]bool) int {
	victim := -1
	for i, c := range committed {
		score, queued := scores[c.ID]
		if !queued || continuations[c.ID] || started[c.ID] {
			continue
		}
		if victim < 0 || score <= scores[committed[victim].ID] {
			victim = i
		}
	}
	return victim
}

func overlapsKept(files []string, committed []FleetCandidate, victim int) bool {
	held := map[string]bool{}
	for i, c := range committed {
		if i == victim {
			continue
		}
		for _, f := range c.Files {
			held[filepath.Clean(f)] = true
		}
	}
	for _, f := range files {
		if held[filepath.Clean(f)] {
			return true
		}
	}
	return false
}

func RecordPreemption(evolveDir string, p Preemption, now time.Time) error {
	return ledger.New(evolveDir).AppendLifecycle(context.Background(), ledger.LifecycleRecord{
		TS:      now.UTC().Format(time.RFC3339),
		Action:  "preempt",
		TaskID:  p.New,
		Message: p.Line(),
	})
}
