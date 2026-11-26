package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMover_Promote_DeliveredMovesAClaimedItemToTheFlatDeliveredDir(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 598, "t1.json"), `{"id":"t1"}`)
	rec := &recordingAppender{}
	landedAsked := false
	m := New(inbox, rec, WithLanded(func(string) (bool, error) { landedAsked = true; return false, nil }))

	res, err := m.Promote("t1", "delivered", PromoteOpts{Cycle: "598", CommitSHA: "deadbeef00"})

	want := filepath.Join(inbox, "delivered", "t1.json")
	if err != nil || res.NoOp || res.DestPath != want {
		t.Fatalf("res = %+v, err = %v; want a move to %s", res, err, want)
	}
	if _, statErr := os.Stat(want); statErr != nil {
		t.Errorf("the item must sit in delivered/: %v", statErr)
	}
	if landedAsked {
		t.Error("a delivery is not a landing: the landing probe must not run")
	}
	if len(rec.records) != 1 || rec.records[0].Message != ".evolve/inbox/processing/t1.json → "+want+": ship-promote-delivered" {
		t.Errorf("ledger = %+v", rec.records)
	}
}
