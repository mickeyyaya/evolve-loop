package landing

import "strings"

type pushFailure uint8

const (
	pushRace pushFailure = iota
	pushTransport
	pushPolicy
)

var policyMarkers = []string{
	"protected branch", "hook declined", "gh006", "gh013", "permission to ", "permission denied",
	"not allowed to push", "push declined", "refusing to allow", "returned error: 403", "authentication failed",
	"read-only",
}

var transportMarkers = []string{
	"internal server error", "bad gateway", "service unavailable", "gateway time", "returned error: 5",
	"returned error: 429", "rpc failed", "could not resolve host", "couldn't connect", "connection reset",
	"connection refused", "connection timed out", "connection closed", "operation timed out", "early eof",
	"remote end hung up",
}

func classifyPush(stderr string) pushFailure {
	text := strings.ToLower(stderr)
	switch {
	case containsAny(text, policyMarkers):
		return pushPolicy
	case containsAny(text, transportMarkers):
		return pushTransport
	}
	return pushRace
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func stderrDetail(stderr string) string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	joined := strings.Join(lines, " | ")
	if len(joined) > maxStderrDetail {
		joined = joined[:maxStderrDetail] + "…"
	}
	return joined
}

const maxStderrDetail = 300
