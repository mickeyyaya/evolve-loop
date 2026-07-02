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
// of concurrent lanes — clamped to a minimum of 1 so the loop always retains
// its sequential fallback. Every benched family WARNs on warn, naming the
// family AND the reason, so the operator sees why capacity dropped. Zero
// benches: count passes through unchanged with zero output.
func QuotaAwareCount(count int, benched map[string]string, warn io.Writer) int {
	if len(benched) == 0 {
		return count
	}
	families := make([]string, 0, len(benched))
	for fam := range benched {
		families = append(families, fam)
	}
	sort.Strings(families) // deterministic WARN order
	effective := count
	for _, fam := range families {
		next := effective - 1
		if next < 1 {
			next = 1
		}
		fmt.Fprintf(warn, "[loop] WARN: fleet: quota bench on CLI family %q (%s): wave count %d -> %d (min 1)\n",
			fam, benched[fam], effective, next)
		effective = next
	}
	return effective
}
