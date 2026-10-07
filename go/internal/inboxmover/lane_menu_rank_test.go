package inboxmover

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRankLaneMenu_RanksTheReadyListAgainstTheWholeQueue(t *testing.T) {
	root := t.TempDir()
	writeTask(t, filepath.Join(root, ".evolve", "inbox"), "blocker", nil)
	queue := []inboxbatch.Item{
		{ID: "plain", Weight: 0.55, PriorityClass: "correctness"},
		{ID: "blocker", Weight: 0.5, PriorityClass: "correctness"},
		{ID: "waits", Weight: 0.9, PriorityClass: "stability", Deps: []string{"blocker"}},
	}
	rank := inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}
	var got RankedLaneMenu = RankLaneMenu(Options{ProjectRoot: root}, queue, nil, rank)
	if want := PartitionLaneMenu(Options{ProjectRoot: root}, queue, nil); !reflect.DeepEqual(got.LaneMenu, want) {
		t.Fatalf("the menu is PartitionLaneMenu's: %+v, want %+v", got.LaneMenu, want)
	}
	if !reflect.DeepEqual(got.Ranked, rank.Order(got.Ready, queue)) {
		t.Fatalf("Ranked is the rank of the ready list against the whole queue")
	}
	var ids []string
	for _, r := range got.Ranked {
		ids = append(ids, r.Item.ID)
	}
	if !reflect.DeepEqual(ids, []string{"blocker", "plain"}) || got.Ranked[0].Breakdown.Facts.Unblocks != 1 {
		t.Errorf("the waiting dependent lifts its lighter blocker: %v", ids)
	}
}
