package fleet

import (
	"fmt"
	"io"
	"sort"
)

// QuotaAwareCount shrinks count by one lane per benched family (family → bench reason), down to minLanes clamped to [1, count].
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
