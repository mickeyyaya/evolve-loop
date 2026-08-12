package inboxmover

// closesmarker.go — the builder-authored closure marker parser.
//
// Consuming an inbox item used to be a separate act from the ship that closed
// it, so forgetting was always possible: #453 landed
// schema-aligned-salvage-layer without consuming its item and wave cycle-1448
// re-picked already-shipped work as live scope. The cure is to let the landing
// itself carry the closure claim — a line-anchored `Closes-Inbox: <id>` in
// build-report.md, unioned into promoteInbox's committed set under the EXACT
// pre-existing cycle-598 landing gate.
//
// Why a marker and not diff inference: an item's `connects_to` is a HINT, not
// an acceptance predicate, so inferring closure from touched paths would
// consume items an unrelated diff merely brushed past. Over-consumption is
// silent data loss; under-consumption only costs a bookkeeping cycle. Closure
// is therefore an explicit, line-anchored, auditor-checkable assertion.

import (
	"regexp"
	"strings"
)

// closesInboxMarker is the marker keyword, matched case-insensitively at the
// start of a line (after optional indent and an optional markdown bullet).
const closesInboxMarker = "closes-inbox:"

// inboxIDPattern is the id shape an inbox filename can take. Anything else on a
// marker line — prose spillover, placeholders like `<id>` — is dropped rather
// than guessed at, so a documented example never consumes a real item.
var inboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ClosesInboxIDs returns the inbox ids a build report declares this landing
// closes, deduped in first-seen order, or nil when the report claims none.
//
// LINE-ANCHORED BY CONTRACT: only a line whose first non-blank content — after
// an optional markdown bullet — is the marker contributes ids. A mid-sentence
// mention of the marker, including this package's own documentation of it,
// contributes nothing. That anti-false-positive half is why a
// substring/regex-anywhere implementation is not a valid implementation.
func ClosesInboxIDs(body []byte) []string {
	var ids []string
	for _, line := range strings.Split(string(body), "\n") {
		rest, ok := cutClosesInboxMarker(line)
		if !ok {
			continue
		}
		for _, field := range strings.Split(rest, ",") {
			id := strings.Trim(strings.TrimSpace(field), "`")
			if inboxIDPattern.MatchString(id) {
				ids = append(ids, id)
			}
		}
	}
	return dedupeIDs(ids)
}

// cutClosesInboxMarker reports whether line is a marker line and returns the
// comma-separated id list that follows the colon.
func cutClosesInboxMarker(line string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	// One optional markdown bullet, so the marker survives being written as a
	// list item in a handoff section.
	if len(s) > 0 && strings.ContainsRune("-*+", rune(s[0])) {
		s = strings.TrimSpace(s[1:])
	}
	if len(s) < len(closesInboxMarker) || !strings.EqualFold(s[:len(closesInboxMarker)], closesInboxMarker) {
		return "", false
	}
	return s[len(closesInboxMarker):], true
}
