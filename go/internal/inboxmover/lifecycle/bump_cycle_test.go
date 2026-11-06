package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestBumpWith_ACycleChargesTheItemOnceAndALaterCycleChargesAgain(t *testing.T) {
	path := filepath.Join(newInbox(t), "item.json")
	writeItem(t, path, `{"id":"x"}`)
	for _, step := range []struct{ cycle, want int }{{7, 1}, {7, 1}, {8, 2}, {8, 2}} {
		got, err := bumpWith(path, failureRecord{reason: "r", cycle: step.cycle}, nil)
		if err != nil || got != step.want {
			t.Fatalf("cycle %d: count = %d, err = %v, want %d", step.cycle, got, err, step.want)
		}
	}
	if got, err := bumpWith(path, failureRecord{reason: "r"}, nil); err != nil || got != 3 {
		t.Fatalf("cycle 0 always bumps: count = %d, err = %v, want 3", got, err)
	}
	if got := string(readJSON(t, path)["last_failure_cycle"]); got != "8" {
		t.Errorf("last_failure_cycle = %s, want 8", got)
	}
}
