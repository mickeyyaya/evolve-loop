package panewatch

import (
	"testing"
	"time"
)

func TestTracker_ShellCountChangeRepublishesWithoutProgress(t *testing.T) {
	tr := NewTracker(identity())
	tr.Observe(Frame{Hash: "h1", Busy: true}, t0)
	later := t0.Add(time.Minute)
	snap, changed := tr.Observe(Frame{Hash: "h1", Busy: true, Shells: 2}, later)
	if !changed || snap.Shells != 2 || snap.ProgressAt != t0 {
		t.Errorf("a shell count change must republish without progress: changed=%v snap=%+v", changed, snap)
	}
}
