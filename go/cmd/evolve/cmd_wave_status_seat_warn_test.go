package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWaveStatus_SeatHealth_AnUnreadablePaneWatchFileWarnsAndShowsNoSeat(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	if err := os.WriteFile(filepath.Join(rig.runDir, "tdd-pane-watch.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	seats, warnings := collectWaveSeats(filepath.Join(rig.plane.root, ".evolve"), waveTestClock)

	if len(seats) != 0 {
		t.Errorf("seats = %+v, want none for an unreadable snapshot", seats)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "seat health") {
		t.Errorf("warnings = %v, want one seat health warning", warnings)
	}
}
