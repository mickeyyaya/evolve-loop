package closesinbox

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

func IDs(body []byte) []string {
	return markerIDs(body, closesInboxMarker)
}

func RefIDs(body []byte) []string {
	return markerIDs(body, refsInboxMarker)
}

func Declares(body []byte) bool {
	declared := false
	visit := func(rest string) {
		declared = declared || len(cleanFields(rest)) > 0
	}
	scanMarkerLines(body, closesInboxMarker, visit)
	scanMarkerLines(body, refsInboxMarker, visit)
	return declared
}

func markerIDs(body []byte, marker string) []string {
	var ids []string
	scanMarkerLines(body, marker, func(rest string) {
		for _, id := range cleanFields(rest) {
			if !strings.EqualFold(id, noInboxIDs) {
				ids = append(ids, id)
			}
		}
	})
	return dedupe(ids)
}

func cleanFields(rest string) []string {
	var fields []string
	for _, field := range strings.Split(rest, ",") {
		id := strings.Trim(strings.TrimSpace(field), "`")
		if inboxIDPattern.MatchString(id) {
			fields = append(fields, id)
		}
	}
	return fields
}

func scanMarkerLines(body []byte, marker string, visit func(rest string)) {
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
		if rest, ok := cutMarker(line, marker); ok {
			visit(rest)
		}
	}
}

func cutMarker(line, marker string) (string, bool) {
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

func dedupe(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
