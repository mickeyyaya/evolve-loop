package closesinbox

import (
	"regexp"
	"strings"
)

const closesInboxMarker = "closes-inbox:"

var inboxIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func IDs(body []byte) []string {
	var ids []string
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
	return dedupe(ids)
}

func cutClosesInboxMarker(line string) (string, bool) {
	s := strings.TrimSuffix(line, "\r")
	if len(s) > 0 && (s[0] == '-' || s[0] == '*' || s[0] == '+') {
		s = s[1:]
		if len(s) == 0 || (s[0] != ' ' && s[0] != '\t') {
			return "", false
		}
		s = strings.TrimLeft(s, " \t")
	}
	if len(s) < len(closesInboxMarker) || !strings.EqualFold(s[:len(closesInboxMarker)], closesInboxMarker) {
		return "", false
	}
	return s[len(closesInboxMarker):], true
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
