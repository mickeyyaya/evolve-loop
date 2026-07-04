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
//   - Input order does not affect the winner among distinct versions.
//   - A versioned id always outranks an unversioned one.
//   - Ties (equal versions, or all-unversioned) keep the first-listed id — a
//     deterministic fallback to classifier/original order, never a crash.
//   - Empty/nil input returns "".
func NewestInLineage(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	best := ids[0]
	bestV := parseVersion(best)
	for _, id := range ids[1:] {
		v := parseVersion(id)
		if newerVersion(v, bestV) {
			best = id
			bestV = v
		}
	}
	return best
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
