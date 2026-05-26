package failureadapter

import "time"

// ListPendingByClass returns the subset of entries whose Classification
// matches target AND which are still within their retention window
// (per the same isNonExpired rule the Decide() kernel uses).
//
// Pure function; no I/O. Input order is preserved. Intended for the
// operator-facing `evolve guard list-audit-fails` subcommand and any
// other read-only enumeration of pending failures.
//
// Use CodeAuditFail to surface the "16 non-expired code-audit-fail"
// entries the dispatcher's retro phase counts in fluent-mode advisories.
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
