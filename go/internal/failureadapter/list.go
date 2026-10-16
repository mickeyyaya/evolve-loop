package failureadapter

import "time"

func ListPendingByClass(entries []Entry, target Classification, now time.Time) []Entry {
	if now.IsZero() {
		now = time.Now()
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Classification != target {
			continue
		}
		if !isNonExpired(e, now) {
			continue
		}
		out = append(out, e)
	}
	return out
}
