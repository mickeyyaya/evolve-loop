package modelquery

import (
	"regexp"
	"strconv"
	"strings"
)

// versionToken matches the first dotted numeric run in a model id (e.g. "4.10"
// in "opus-4.10", "3.1" in "Gemini 3.1 Pro (High)"). Capability/effort markers
// like "-mini", "(High)", "(Thinking)" carry no digits, so they never leak
// into the extracted version.
var versionToken = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)*`)

// NewestInLineage returns the numerically-newest model id from a set of ids
// assumed to belong to one lineage (the D2 "newest-wins" comparator of the
// latest-model-preference feature). Comparison is component-wise numeric — so
// "opus-4.10" outranks "opus-4.9", which a naive lexicographic compare gets
// wrong. The full id string is returned unmodified.
//
// Contract:
//   - The version is read with any date-shaped run (dateRun) removed, so a
//     snapshot date is never compared as a version number.
//   - Input order does not affect the winner among distinct versions.
//   - A versioned id always outranks an unversioned one.
//   - Equal versions that are BOTH dated are ordered by calendar date.
//   - Other ties (equal versions with at most one side dated, or
//     all-unversioned) keep the first-listed id — a deterministic fallback to
//     classifier/original order, never a crash.
//   - Empty/nil input returns "".
func NewestInLineage(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	best := ids[0]
	for _, id := range ids[1:] {
		if newer(id, best) {
			best = id
		}
	}
	return best
}

// newer reports whether id a is strictly newer than id b within one lineage.
// The numeric version decides first, read with the date run removed so a
// snapshot date is never mistaken for a version number. The date only breaks
// a tie between equal versions, and only when BOTH ids are dated: an undated
// id carries no date to compare against, so neither side displaces the other
// and the caller keeps the incumbent — an uncertain identity never
// substitutes. This is a strict partial order (lexicographic on version, then
// date), so a scan's winner is either its starting id or strictly newer than it.
func newer(a, b string) bool {
	va, vb := parseVersion(withoutDate(a)), parseVersion(withoutDate(b))
	if newerVersion(va, vb) {
		return true
	}
	if newerVersion(vb, va) {
		return false
	}
	da, db := parseDate(a), parseDate(b)
	return da.ok && db.ok && da.value > db.value
}

// version is a parsed model version. ok is false when the id carries no numeric
// version token at all (e.g. "latest", "stable").
type version struct {
	ok    bool
	parts []int
}

// parseVersion extracts the first dotted numeric run from id into per-component
// integers. A missing token yields ok=false.
func parseVersion(id string) version {
	tok := versionToken.FindString(id)
	if tok == "" {
		return version{}
	}
	fields := strings.Split(tok, ".")
	parts := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return version{}
		}
		parts = append(parts, n)
	}
	return version{ok: true, parts: parts}
}

// newerVersion reports whether a is strictly newer than b. A versioned id beats
// an unversioned one; equal or both-unversioned is not "strictly newer", so the
// caller keeps the earlier (first-listed) id.
func newerVersion(a, b version) bool {
	if a.ok != b.ok {
		return a.ok // versioned beats unversioned
	}
	if !a.ok {
		return false // both unversioned → keep first-listed
	}
	return compareParts(a.parts, b.parts) > 0
}

// dateVersion is a parsed calendar-date snapshot token (dateRun: YYYY[-MM[-DD]]).
// ok is false when the id carries no date-shaped run.
type dateVersion struct {
	ok    bool
	value int // YYYYMMDD; a missing month/day is treated as 0
}

// parseDate extracts the dateRun token from id into a comparable YYYYMMDD
// integer. A missing token, or a non-numeric component (impossible given
// dateRun's own character class but checked for defensiveness), yields ok=false.
func parseDate(id string) dateVersion {
	tok := dateRun.FindString(id)
	if tok == "" {
		return dateVersion{}
	}
	fields := strings.Split(tok, "-")
	year, err := strconv.Atoi(fields[0])
	if err != nil {
		return dateVersion{}
	}
	month, day := 0, 0
	if len(fields) > 1 {
		if month, err = strconv.Atoi(fields[1]); err != nil {
			return dateVersion{}
		}
	}
	if len(fields) > 2 {
		if day, err = strconv.Atoi(fields[2]); err != nil {
			return dateVersion{}
		}
	}
	return dateVersion{ok: true, value: year*10000 + month*100 + day}
}

// compareParts compares two numeric version component slices, treating a
// missing trailing component as 0 (so "4" == "4.0"). Returns >0 if a>b, <0 if
// a<b, 0 if equal.
func compareParts(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
}
