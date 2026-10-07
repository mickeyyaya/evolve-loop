package triagecap

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func rankFixture(t *testing.T) string {
	t.Helper()
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	fixtures.MustWrite(t, filepath.Join(inbox, "a.json"), `{"id":"heavy-security","weight":0.6,"priority_class":"security","files":["go/internal/a/a.go"]}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "b.json"), `{"id":"light-correctness","weight":0.55,"priority_class":"correctness","files":["go/internal/b/b.go"]}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "c.json"), `{"id":"blocker","weight":0.5,"priority_class":"stability","files":["go/internal/c/c.go"]}`)
	fixtures.MustWrite(t, filepath.Join(inbox, "d.json"), `{"id":"waits-on-blocker","weight":0.9,"priority_class":"stability","deps":["blocker"]}`)
	return evolveDir
}

func rankOrderIDs(t *testing.T, evolveDir string, in inboxrank.Inputs) []string {
	t.Helper()
	queue, _, err := inboxbatch.LoadDir(filepath.Join(evolveDir, "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	ready := inboxmover.PartitionLaneMenu(readOnlyLifecycle(evolveDir), queue, noProtectedSurface).Ready
	var ids []string
	for _, r := range in.Order(ready, queue) {
		ids = append(ids, r.Item.ID)
	}
	return ids
}

func TestReadInboxBacklog_IsTheRanksOrderOverTheReadyListAgainstTheWholeQueue(t *testing.T) {
	evolveDir := rankFixture(t)
	in := inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: time.Now()}
	want := []string{"blocker", "light-correctness", "heavy-security"}
	if got := rankOrderIDs(t, evolveDir, in); !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture: the rank orders %v, want %v", got, want)
	}
	if got := ids(ReadInboxBacklog(evolveDir, noProtectedSurface)); !reflect.DeepEqual(got, want) {
		t.Errorf("ReadInboxBacklog = %v, want inboxrank.Order's %v (a waiting dependent lifts its blocker; the class reorders near-ties)", got, want)
	}
}

func TestReadInboxBacklog_RanksWithTheEvolveDirsPolicyAndRecurrenceLedger(t *testing.T) {
	evolveDir := rankFixture(t)
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"inbox_priority":{"class_order":["security","stability","correctness"]}}`)
	fixtures.MustWrite(t, filepath.Join(evolveDir, "recurrence-ledger.json"), `{"entries":{"blocker":{"pattern":"blocker","count":5}}}`)
	got := ids(ReadInboxBacklog(evolveDir, noProtectedSurface))
	if want := []string{"blocker", "heavy-security", "light-correctness"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadInboxBacklog = %v, want %v under the evolve dir's own class order and ledger", got, want)
	}
}

func TestSelectFleetWidthTopN_KeepsTheBacklogsRankOrder(t *testing.T) {
	ranked := []FleetCandidate{
		{ID: "first", Weight: 0.1, Files: []string{"go/internal/a/a.go"}},
		{ID: "second", Weight: 0.9, Files: []string{"go/internal/a/a.go"}},
		{ID: "third", Weight: 0.5, Files: []string{"go/internal/b/b.go"}},
	}
	if got := ids(SelectFleetWidthTopN(ranked, 2)); !reflect.DeepEqual(got, []string{"first", "third"}) {
		t.Errorf("count=2 seeded %v, want [first third]: the first-ranked disjoint candidates, whatever their weight", got)
	}
	if got := ids(SelectFleetWidthTopN(ranked, 1)); !reflect.DeepEqual(got, []string{"first"}) {
		t.Errorf("count<2 seeded %v, want the first-ranked candidate", got)
	}
}

func TestWidenTopNToFleetWidth_BackfillsInTheBacklogsRankOrder(t *testing.T) {
	committed := []FleetCandidate{{ID: "kept", Files: []string{"go/internal/k/k.go"}}}
	ranked := []FleetCandidate{{ID: "ranked-first", Weight: 0.1, Files: []string{"go/internal/a/a.go"}}, {ID: "heavier", Weight: 0.9, Files: []string{"go/internal/b/b.go"}}}
	if got := ids(WidenTopNToFleetWidth(committed, ranked, 2)); !reflect.DeepEqual(got, []string{"kept", "ranked-first"}) {
		t.Errorf("backfill = %v, want [kept ranked-first]", got)
	}
}

func TestExpandWithClusterMates_TakesMatesInTheBacklogsRankOrder(t *testing.T) {
	selection := []FleetCandidate{{ID: "rep", Files: []string{"go/internal/a/a.go"}}}
	ranked := []FleetCandidate{
		{ID: "rep", Files: []string{"go/internal/a/a.go"}},
		{ID: "light-mate", Weight: 0.1, Files: []string{"go/internal/a/a.go"}},
		{ID: "heavy-mate", Weight: 0.9, Files: []string{"go/internal/a/a.go"}},
	}
	menus := ExpandWithClusterMates(selection, ranked, 2)
	if got := ids(menus[0]); !reflect.DeepEqual(got, []string{"rep", "light-mate"}) {
		t.Errorf("lane = %v, want [rep light-mate]: the first-ranked mate fills the lane", got)
	}
}
