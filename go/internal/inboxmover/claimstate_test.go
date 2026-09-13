package inboxmover

// claimstate_test.go — the read-only lifecycle reader the ADR-0100
// declared-effects gate consumes (deliverable/effects.go). This package owns
// the processing/cycle-N/ layout; the gate never re-derives it.

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestLocate_RootClaimedAndAbsent(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	seedItemIn(t, inbox, "pending")
	seedItemIn(t, filepath.Join(inbox, "processing", "cycle-3"), "held")

	var loc Location
	loc, err := Locate(inbox, "pending")
	if err != nil || loc.Cycle != 0 || filepath.Dir(loc.Path) != inbox {
		t.Fatalf("a root item locates as pending (Cycle 0) at its path; got %+v err=%v", loc, err)
	}
	if loc, err := Locate(inbox, "held"); err != nil || loc.Cycle != 3 {
		t.Fatalf("an item under processing/cycle-3/ locates as claimed by cycle 3; got %+v err=%v", loc, err)
	}
	if _, err := Locate(inbox, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an id with no file anywhere is ErrNotFound, got %v", err)
	}
}

// No inbox at all is "nothing to claim", not an IO fault: the gate must not
// fail open on a project that never had an inbox.
func TestLocate_MissingInboxIsNotFound(t *testing.T) {
	if _, err := Locate(filepath.Join(t.TempDir(), "absent"), "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing inbox dir must be ErrNotFound, got %v", err)
	}
}

// The property this slice exists for: the gate sees the effect the persona's
// `evolve inbox-mover claim` performs. Proved against the production writer,
// not a hand-built copy of its layout.
func TestLocate_FindsWhatClaimWrote(t *testing.T) {
	root, inbox := seedInbox(t, "x")
	if _, err := Claim(Options{ProjectRoot: root}, "x", "7"); err != nil {
		t.Fatal(err)
	}
	loc, err := Locate(inbox, "x")
	if err != nil || loc.Cycle != 7 {
		t.Fatalf("Locate must find the item exactly where Claim moved it (cycle 7); got %+v err=%v", loc, err)
	}
	if filepath.Dir(loc.Path) != inboxbatch.ProcessingCycleDir(inbox, 7) {
		t.Fatalf("Claim wrote to %q, which is not the shared layout %q", filepath.Dir(loc.Path), inboxbatch.ProcessingCycleDir(inbox, 7))
	}
}

// Liveness order matches Promote's: a claim outranks a stale root copy of the
// same id, so the gate never reads "pending" for an item a lane holds.
func TestLocate_PrefersTheClaimOverAStaleRootCopy(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	seedItemIn(t, inbox, "dup")
	seedItemIn(t, inboxbatch.ProcessingCycleDir(inbox, 3), "dup")
	if loc, err := Locate(inbox, "dup"); err != nil || loc.Cycle != 3 {
		t.Fatalf("the claim must win over the root copy; got %+v err=%v", loc, err)
	}
}
