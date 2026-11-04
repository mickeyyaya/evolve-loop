package inboxstamps

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type captureSink struct {
	entries []core.BookkeepingEntry
	err     error
}

func (s *captureSink) Add(e core.BookkeepingEntry) error {
	s.entries = append(s.entries, e)
	return s.err
}

func productionPlan() Plan {
	return Plan{
		Kept:   []Stamp{{Path: ".evolve/inbox/k.json", Set: map[string]json.RawMessage{}}},
		Replay: []Stamp{{Path: ".evolve/inbox/r.json", Set: map[string]json.RawMessage{}}},
	}
}

func TestHoldKeptAndHoldReplay_SendTheStampsToTheSinkNotToGit(t *testing.T) {
	t.Parallel()
	sink := &captureSink{}
	plan := productionPlan()
	if err := plan.HoldKept(sink); err != nil {
		t.Fatal(err)
	}
	if err := plan.HoldReplay(sink); err != nil {
		t.Fatal(err)
	}
	want := []core.BookkeepingEntry{
		{Kind: core.BookkeepingInboxStamp, Paths: []string{".evolve/inbox/k.json"}, Message: keptMessage},
		{Kind: core.BookkeepingInboxStamp, Paths: []string{".evolve/inbox/r.json"}, Message: replayMessage},
	}
	if len(sink.entries) != 2 || sink.entries[0].Kind != want[0].Kind || sink.entries[0].Message != want[0].Message || sink.entries[1].Message != want[1].Message ||
		sink.entries[0].Paths[0] != want[0].Paths[0] || sink.entries[1].Paths[0] != want[1].Paths[0] {
		t.Fatalf("entries = %+v, want %+v", sink.entries, want)
	}
}

func TestHoldKeptAndHoldReplay_AnEmptyListNeverReachesTheSink(t *testing.T) {
	t.Parallel()
	sink := &captureSink{}
	plan := Plan{}
	if err := plan.HoldKept(sink); err != nil {
		t.Fatal(err)
	}
	if err := plan.HoldReplay(sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(sink.entries))
	}
}

func TestHoldKept_WrapsASinkRefusal(t *testing.T) {
	t.Parallel()
	plan := productionPlan()
	err := plan.HoldKept(&captureSink{err: errors.New("down")})
	if err == nil || !strings.Contains(err.Error(), "down") {
		t.Fatalf("err = %v, want the sink refusal", err)
	}
}

func TestStampJourney_AFullBatchHoldsEachStampOnceInOrder(t *testing.T) {
	t.Parallel()
	batch := core.NewBookkeepingBatch()
	plan := productionPlan()
	if err := plan.HoldKept(batch); err != nil {
		t.Fatal(err)
	}
	if err := plan.HoldReplay(batch); err != nil {
		t.Fatal(err)
	}
	got := batch.Drain()
	if len(got) != 2 || got[0].Message != keptMessage || got[1].Message != replayMessage || got[0].Kind != core.BookkeepingInboxStamp {
		t.Fatalf("drained %+v", got)
	}
}
