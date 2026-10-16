package inboxmover

import (
	"regexp"
	"strings"
)

const closesInboxMarker = "closes-inbox:"

var inboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

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
