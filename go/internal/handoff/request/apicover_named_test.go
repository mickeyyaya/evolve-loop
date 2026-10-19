package request_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestExportsAreNamed(t *testing.T) {
	var resolve request.PathResolver = func(string) string { return "" }
	if _, err := request.TriageTDD(t.TempDir(), resolve); !errors.Is(err, request.ErrNoCommittedSet) {
		t.Fatalf("empty workspace = %v, want ErrNoCommittedSet", err)
	}
	if _, err := request.TDDBuild(t.TempDir()); !errors.Is(err, request.ErrNoPredicates) {
		t.Fatalf("empty package = %v, want ErrNoPredicates", err)
	}
	if !errors.Is(request.ErrUnresolvedItem, request.ErrUnresolvedItem) {
		t.Fatal("ErrUnresolvedItem must match itself")
	}
	if got := request.ScoutTriage([]string{"a"}).Pair; got != request.PairScoutTriage {
		t.Errorf("pair = %q", got)
	}
	if got := request.TriageTDDFromItems([]inboxbatch.Item{{ID: "a", Acceptance: []string{"x"}}}).Pair; got != request.PairTriageTDD {
		t.Errorf("pair = %q", got)
	}
	if request.PairTDDBuild != "tdd-build" {
		t.Errorf("pair = %q", request.PairTDDBuild)
	}
}
