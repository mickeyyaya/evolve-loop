package fleet

// quota.go — FLEET-AS-POLICY S3(b): the quota-aware wave Count shrink. The
// bench SSOT is clihealth.Store (the bridge writes a bench on every quota
// classification — cycle-283 forensics), so no new probing happens here: the
// caller intersects Store.Active() with the families the wave needs and
// passes only the relevant entries.

import (
	"fmt"
	"io"
	"sort"
)

// QuotaAwareCount returns the effective wave lane count after shrinking for
// benched required CLI families. benched maps each required family to its
// active bench reason (classifier pattern, e.g. "rate_limit"). Each benched
// family shrinks the count by one — a benched family cannot absorb its share
// of concurrent lanes — clamped to the operator's minLanes floor (the asserted
// concurrent-lane budget; default 1 keeps the sequential fallback). A benched
// family that shrinks capacity WARNs "wave count N -> M"; when the min-lanes
// floor absorbs the bench instead (the operator budgeted for it), it WARNs that
// capacity was HELD at the floor — so the operator always sees a benched family
// AND its reason, whether or not it cost a lane. Zero benches: count passes
// through unchanged with zero output.
//
// minLanes is clamped to [1, count]: a floor of 0/absent is the historical
// min-1 behaviour; a floor above the configured count is meaningless (you
// cannot run more lanes than were planned) and clamps down to count.
func QuotaAwareCount(count int, benched map[string]string, minLanes int, warn io.Writer) int {
	if len(benched) == 0 {
		return count
	}
	floor := minLanes
	if floor < 1 {
		floor = 1
	}
	if floor > count {
		floor = count
	}
	families := make([]string, 0, len(benched))
	for fam := range benched {
		families = append(families, fam)
	}
	sort.Strings(families) // deterministic WARN order
	effective := count
	for _, fam := range families {
		next := effective - 1
		if next < floor {
			// The operator's min-lanes floor absorbs this bench: capacity holds.
			// (Inside this arm effective <= floor always, so there is no distinct
			// "would-have-shrunk" vs "already-floored" case to report — one honest
			// message per benched family, naming family + reason + the floor.)
			fmt.Fprintf(warn, "[loop] WARN: fleet: quota bench on CLI family %q (%s): capacity held at %d by fleet.min_lanes floor\n",
				fam, benched[fam], floor)
			next = floor
		} else {
			fmt.Fprintf(warn, "[loop] WARN: fleet: quota bench on CLI family %q (%s): wave count %d -> %d (min %d)\n",
				fam, benched[fam], effective, next, floor)
		}
		effective = next
	}
	return effective
}
