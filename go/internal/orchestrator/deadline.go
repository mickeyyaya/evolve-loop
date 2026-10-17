package orchestrator

import (
	"slices"
	"time"
)

func DeadlineOf(laneTimes []time.Duration, floor time.Duration) time.Duration {
	if len(laneTimes) == 0 {
		return floor
	}
	sorted := slices.Sorted(slices.Values(laneTimes))
	if len(sorted) > 1 {
		sorted = sorted[:len(sorted)-1]
	}
	rank := (95*len(sorted) + 99) / 100
	return max(sorted[rank-1], floor)
}
