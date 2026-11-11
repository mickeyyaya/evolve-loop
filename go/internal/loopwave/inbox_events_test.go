package loopwave

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestClaims_ReleasePublishesInboxReleasedThroughTheEnginesCenter(t *testing.T) {
	h := newHarness(t)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "processing", "cycle-9", "stale-item.json"), map[string]any{"id": "stale-item", "weight": 0.5})

	if err := h.e.Claims().Release("stale-item", "orchestrator: deadline"); err != nil {
		t.Fatalf("Release = %v", err)
	}

	var released []signalcenter.Event
	for _, e := range *h.events {
		if e.Kind == signalcenter.KindInboxReleased {
			released = append(released, e)
		}
	}
	if len(released) != 1 || released[0].Module != signalcenter.ModuleInbox || released[0].Cycle != 9 || released[0].Fields["id"] != "stale-item" || released[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("inbox.released events = %+v, want one INFO inbox event for stale-item at cycle 9", released)
	}
}
