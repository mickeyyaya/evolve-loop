package fleet

import (
	"fmt"
	"io"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func benchLabel(pattern string) string {
	if clihealth.LoginPattern(pattern) {
		return "login bench"
	}
	return "quota bench"
}

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
	sort.Strings(families)
	effective := count
	for _, fam := range families {
		next := effective - 1
		if next < floor {
			fmt.Fprintf(warn, "[loop] WARN: fleet: %s on CLI family %q (%s): capacity held at %d by fleet.min_lanes floor\n",
				benchLabel(benched[fam]), fam, benched[fam], floor)
			next = floor
		} else {
			fmt.Fprintf(warn, "[loop] WARN: fleet: %s on CLI family %q (%s): wave count %d -> %d (min %d)\n",
				benchLabel(benched[fam]), fam, benched[fam], effective, next, floor)
		}
		effective = next
	}
	return effective
}
