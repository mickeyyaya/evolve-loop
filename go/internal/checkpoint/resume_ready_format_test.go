package checkpoint

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

func benchResetISO(t *testing.T, now, benchedUntil time.Time) string {
	t.Helper()
	res, err := quotareset.Compute("", quotareset.Options{Now: func() time.Time { return now }, BenchedUntil: benchedUntil})
	if err != nil || res.Source != "bench" {
		t.Fatalf("the bench reset path = %+v err %v, want source bench", res, err)
	}
	return res.ISO
}

func TestResumeReady_ReadsTheResetTimeTheBenchPathWrites(t *testing.T) {
	t.Parallel()
	taipei := time.FixedZone("+0800", 8*3600)
	written := time.Date(2026, 11, 4, 0, 30, 0, 0, time.UTC)
	future := benchResetISO(t, written, time.Date(2026, 11, 4, 8, 53, 30, 0, taipei))
	if future != "2026-11-04T08:53:30+0800" {
		t.Fatalf("the bench path wrote %q, want the +0800 layout", future)
	}
	if ready, err := ResumeReady(1, 3, future, written); ready || err != nil {
		t.Errorf("ResumeReady(%q) at %s = %v err %v, want false: the reset is still ahead", future, written, ready, err)
	}
	if ready, err := ResumeReady(1, 3, future, written.Add(time.Hour)); !ready || err != nil {
		t.Errorf("ResumeReady(%q) one hour after the reset = %v err %v, want true", future, ready, err)
	}
}

func TestResumeReady_AnUnreadableResetTimeIsReadyWithAWarning(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 4, 0, 30, 0, 0, time.UTC)
	ready, err := ResumeReady(1, 3, "tomorrow morning", now)
	if !ready || err == nil || !strings.Contains(err.Error(), "tomorrow morning") {
		t.Fatalf("ResumeReady(unreadable) = %v err %v, want ready with an error that names the value", ready, err)
	}
	if ready, err := ResumeReady(3, 3, "tomorrow morning", now); ready || err != nil {
		t.Errorf("a spent budget = %v err %v, want not ready and no warning", ready, err)
	}
}
