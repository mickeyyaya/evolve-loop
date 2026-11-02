package supersede_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/supersede"
)

func TestCheck_ConsoleClaimSupersedesAndOwnClaimDoesNot(t *testing.T) {
	var read supersede.Reader = func(string) (supersede.Reading, error) {
		return supersede.Reading{Cycle: 9, Route: ""}, nil
	}
	got, err := supersede.Check(read, "item", 3)
	if err != nil || got != (supersede.Result{Verdict: supersede.Superseded, Claim: supersede.Reading{Cycle: 9}}) {
		t.Errorf("another holder = %+v, %v, want superseded with the claim", got, err)
	}
	own := func(string) (supersede.Reading, error) { return supersede.Reading{Cycle: 3}, nil }
	if got, err := supersede.Check(own, "item", 3); err != nil || got.Verdict != supersede.None {
		t.Errorf("own claim = %+v, %v, want none", got, err)
	}
}

func TestCheck_AReadFailureIsAnErrorNotASupersede(t *testing.T) {
	boom := errors.New("unreadable")
	read := func(string) (supersede.Reading, error) { return supersede.Reading{Cycle: 9}, boom }
	got, err := supersede.Check(read, "item", 3)
	if !errors.Is(err, boom) || got.Verdict == supersede.Superseded {
		t.Errorf("failed read = %+v, %v, want the wrapped error and no supersede", got, err)
	}
}

func TestInboxReader_ReadsTheClaimAndTheRouteOfTheItemFile(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	dir := inboxbatch.ProcessingCycleDir(inbox, 5)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.json"), []byte(`{"id":"x","route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	read := supersede.InboxReader(inbox)
	got, err := read("x")
	if err != nil || got != (supersede.Reading{Cycle: 5, Route: "console-manual"}) {
		t.Errorf("read = %+v, %v, want cycle 5 routed to the console", got, err)
	}
	if _, err := read("missing"); err == nil {
		t.Errorf("an item that is in no directory read without an error")
	}
}

func TestInboxReader_AnUnreadableOrMalformedItemFileIsAnError(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	dir := inboxbatch.ProcessingCycleDir(inbox, 5)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "y.json")
	if err := os.WriteFile(path, []byte(`{"id":"y","route":["not","a","string"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := supersede.InboxReader(inbox)("y"); err == nil {
		t.Errorf("a route that is not a string read without an error")
	}
}
