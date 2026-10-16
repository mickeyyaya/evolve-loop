package carryover

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

func refreshExpiry(t *cyclestate.CarryoverTodo, stamp string) bool {
	if stamp > t.ExpiresAt {
		t.ExpiresAt = stamp
		return true
	}
	return false
}

func defaultExpiresAt(now time.Time) string {
	return now.Add(failurelog.DefaultCarryoverBackfillTTL).Format(time.RFC3339)
}
