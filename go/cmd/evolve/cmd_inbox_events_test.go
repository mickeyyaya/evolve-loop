package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestInboxLifecycle_TheHostClaimerPublishesClaimedThroughTheRootCenter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-a.json"), []byte(`{"id":"item-a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New()
	var claimed []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindInboxClaimed {
			claimed = append(claimed, e)
		}
	})

	if err := hostInboxClaimer(nil, center, nil)(inbox, 8, []string{"item-a"}); err != nil {
		t.Fatalf("hostInboxClaimer: %v", err)
	}

	if len(claimed) != 1 || claimed[0].Module != signalcenter.ModuleInbox || claimed[0].Cycle != 8 || claimed[0].Fields["id"] != "item-a" {
		t.Errorf("inbox.claimed events = %+v, want one inbox event for item-a at cycle 8", claimed)
	}
}
