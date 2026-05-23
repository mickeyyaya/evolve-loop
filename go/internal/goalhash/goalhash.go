// Package goalhash computes a stable hash of a /evolve-loop goal string.
//
// Byte-exact port of the bash pipeline at
// legacy/scripts/lifecycle/intent-batch-resolve.sh:
//
//	normalize_goal() {
//	    local raw="$1"
//	    printf '%s\n' "$raw" | tr '[:upper:]' '[:lower:]' \
//	        | tr -s '[:space:]' ' ' \
//	        | sed 's/^ //; s/ $//'
//	}
//	sha256_of() {
//	    printf '%s' "$text" | sha256sum | awk '{print $1}'
//	}
//	GOAL_HASH=$(sha256_of "$(normalize_goal "$GOAL_TEXT")")
//
// Equivalence is load-bearing: the goalHash recorded in
// state.json:currentBatch.goalHash is the cross-session identity of a
// goal. Cross-language drift would cause same-goal cycles to be treated
// as different batches, breaking incremental-intent delta detection
// (EVOLVE_INTENT_DELTA) and resume continuity.
package goalhash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// Normalize lowercases the goal, squeezes any run of unicode whitespace
// down to a single ASCII space, and strips leading/trailing whitespace.
// Equivalent to:
//
//	printf '%s\n' "$raw" | tr '[:upper:]' '[:lower:]' \
//	    | tr -s '[:space:]' ' ' | sed 's/^ //; s/ $//'
//
// Note: the bash pipeline begins with `printf '%s\n'` which appends a
// trailing newline; `tr -s '[:space:]' ' '` then folds that newline
// into a trailing single-space, and `sed 's/ $//'` removes it. The Go
// implementation produces the same final string without the
// intermediate newline.
func Normalize(raw string) string {
	lower := strings.ToLower(raw)
	// Replace any whitespace run (incl. tab/newline) with a single ASCII space.
	var b strings.Builder
	b.Grow(len(lower))
	prevSpace := false
	for _, r := range lower {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

// Compute returns the lower-hex 64-char SHA256 of Normalize(goal).
// Matches the bash `sha256_of "$(normalize_goal "$GOAL_TEXT")"` chain.
func Compute(goal string) string {
	sum := sha256.Sum256([]byte(Normalize(goal)))
	return hex.EncodeToString(sum[:])
}

// Short returns the first 8 chars of Compute(goal). Convenience for
// log-display and batch-ID derivation; callers that need the full
// canonical hash MUST use Compute (state.json:currentBatch.goalHash
// stores the full 64-char value).
func Short(goal string) string {
	return Compute(goal)[:8]
}
