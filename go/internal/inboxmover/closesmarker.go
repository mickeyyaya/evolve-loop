package inboxmover

import (
	"regexp"
	"strings"
)

// closesInboxMarker is matched case-insensitively at a line start, after an optional markdown bullet.
const closesInboxMarker = "closes-inbox:"

// inboxIDPattern drops anything that is not an id, such as a `<id>` placeholder,
// so a documented example never consumes a real item.
var inboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ClosesInboxIDs returns the deduped ids of a build report's line-anchored Closes-Inbox markers, or nil.
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

// cutClosesInboxMarker returns the id list after a line-start marker. A mention
// mid-sentence is not a marker, because over-consuming an item loses data.
func cutClosesInboxMarker(line string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if len(s) > 0 && strings.ContainsRune("-*+", rune(s[0])) {
		s = strings.TrimSpace(s[1:])
	}
	if len(s) < len(closesInboxMarker) || !strings.EqualFold(s[:len(closesInboxMarker)], closesInboxMarker) {
		return "", false
	}
	return s[len(closesInboxMarker):], true
}
