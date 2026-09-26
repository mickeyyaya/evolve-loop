package inboxmover

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestClaimLaneScope_AlreadyClaimedIdIsNotANotFoundClaim(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claimed", "unclaimed"} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := signalcenter.New()
	var codes []signalcenter.Code
	c.Subscribe(func(e signalcenter.Event) { codes = append(codes, e.Code) })
	opts := Options{ProjectRoot: root, Stderr: io.Discard, Signals: c}
	if _, err := Claim(opts, "claimed", "7"); err != nil {
		t.Fatalf("lane claim: %v", err)
	}
	claimed, err := ClaimLaneScope(opts, 7, []string{"claimed", "unclaimed"})
	if err != nil {
		t.Fatalf("ClaimLaneScope: %v", err)
	}
	if len(claimed) != 2 {
		t.Errorf("both ids are in processing/cycle-7 afterwards: %v", claimed)
	}
	if len(codes) != 0 {
		t.Errorf("no fault for an id the lane already claimed: %v", codes)
	}
	for _, id := range []string{"claimed", "unclaimed"} {
		if _, err := FindFileByTaskID(filepath.Join(inbox, "processing", "cycle-7"), id); err != nil {
			t.Errorf("%s not in processing/cycle-7: %v", id, err)
		}
	}
}
