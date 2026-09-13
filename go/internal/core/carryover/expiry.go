package carryover

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// refreshExpiry moves a todo's stamp forward when stamp is later, and reports
// whether it wrote. The compare is lexical: every stamp is RFC3339 UTC at
// second precision ("…Z"), where lexical order is chronological order; an
// empty stamp never wins.
func refreshExpiry(t *cyclestate.CarryoverTodo, stamp string) bool {
	if stamp > t.ExpiresAt {
		t.ExpiresAt = stamp
		return true
	}
	return false
}

// defaultExpiresAt is the stamp a cycle-workspace mint (memo, prescription)
// carries: the caller's now plus the boot prune's backfill TTL — the ONE
// spelling of a rule two files used to hold.
func defaultExpiresAt(now time.Time) string {
	return now.Add(failurelog.DefaultCarryoverBackfillTTL).Format(time.RFC3339)
}
