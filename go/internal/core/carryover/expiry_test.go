package carryover

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

func TestBoundedExpiry_KeepsAnEarlierStampAndCapsALaterOneAtTheTTL(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	ceiling := now.Add(failurelog.DefaultCarryoverBackfillTTL).Format(time.RFC3339)
	for stamp, want := range map[string]string{
		"2026-10-22T00:00:00Z": "2026-10-22T00:00:00Z",
		ceiling:                ceiling,
		"2058-03-30T00:00:00Z": ceiling,
	} {
		if got := BoundedExpiry(stamp, now); got != want {
			t.Errorf("BoundedExpiry(%s) = %s, want %s", stamp, got, want)
		}
	}
}
