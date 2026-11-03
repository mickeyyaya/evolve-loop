package inboxmover

import (
	"regexp"
	"strings"
)

const (
	closesInboxMarker = "closes-inbox:"
	refsInboxMarker   = "refs-inbox:"
	noInboxIDs        = "none"
)

var inboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func ClosesInboxIDs(body []byte) []string {
	return markerIDs(body, closesInboxMarker)
}

func RefsInboxIDs(body []byte) []string {
	return markerIDs(body, refsInboxMarker)
}

func DeclaresInboxTrailer(body []byte) bool {
	declared := false
	scanMarkerLines(body, func(string) { declared = true }, closesInboxMarker, refsInboxMarker)
	return declared
}

func markerIDs(body []byte, marker string) []string {
	var ids []string
	scanMarkerLines(body, func(rest string) {
		for _, field := range strings.Split(rest, ",") {
			id := strings.Trim(strings.TrimSpace(field), "`")
			if inboxIDPattern.MatchString(id) && !strings.EqualFold(id, noInboxIDs) {
				ids = append(ids, id)
			}
		}
	}, marker)
	return dedupeIDs(ids)
}

func scanMarkerLines(body []byte, visit func(rest string), markers ...string) {
	var inFence bool
	var fenceChar byte
	for _, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if !inFence {
			if strings.HasPrefix(line, "```") {
				inFence = true
				fenceChar = '`'
				continue
			}
			if strings.HasPrefix(line, "~~~") {
				inFence = true
				fenceChar = '~'
				continue
			}
		} else {
			if fenceChar == '`' && strings.HasPrefix(line, "```") {
				inFence = false
				continue
			}
			if fenceChar == '~' && strings.HasPrefix(line, "~~~") {
				inFence = false
				continue
			}
			continue
		}
		for _, marker := range markers {
			if rest, ok := cutInboxMarker(line, marker); ok {
				visit(rest)
				break
			}
		}
	}
}

func cutClosesInboxMarker(line string) (string, bool) {
	return cutInboxMarker(line, closesInboxMarker)
}

func cutInboxMarker(line, marker string) (string, bool) {
	s := strings.TrimSuffix(line, "\r")
	if len(s) > 0 && (s[0] == '-' || s[0] == '*' || s[0] == '+') {
		s = s[1:]
		if len(s) == 0 || (s[0] != ' ' && s[0] != '\t') {
			return "", false
		}
		s = strings.TrimLeft(s, " \t")
	}
	if len(s) < len(marker) || !strings.EqualFold(s[:len(marker)], marker) {
		return "", false
	}
	return s[len(marker):], true
}
