package launchoutcome

import (
	"regexp"
	"strings"
)

const ArtifactTimeoutMarker = "artifact-timeout: "

type TimeoutCause string

const (
	TimeoutContextCancelled  TimeoutCause = "context_cancelled"
	TimeoutPaneLost          TimeoutCause = "pane_lost"
	TimeoutDetectorError     TimeoutCause = "completion_detector_error"
	TimeoutSubmitWedged      TimeoutCause = "submit_wedged"
	TimeoutTransientUpstream TimeoutCause = "transient_upstream"
	TimeoutReviewStop        TimeoutCause = "review_stop"
	TimeoutReviewPause       TimeoutCause = "review_pause"
	TimeoutIncomplete        TimeoutCause = "incomplete"
)

func (c TimeoutCause) Known() bool {
	switch c {
	case TimeoutContextCancelled, TimeoutPaneLost, TimeoutDetectorError, TimeoutSubmitWedged,
		TimeoutTransientUpstream, TimeoutReviewStop, TimeoutReviewPause,
		TimeoutIncomplete:
		return true
	}
	return false
}

const (
	maxCauseRunes   = 300
	maxSummaryRunes = 1024
)

func firstDiagnosticLine(stderr string) string {
	last := ""
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[bridge]") {
			return boundCause(line)
		}
		last = line
	}
	return boundCause(last)
}

func boundCause(line string) string {
	if runes := []rune(line); len(runes) > maxCauseRunes {
		return string(runes[:maxCauseRunes]) + "…"
	}
	return line
}

func ArtifactTimeoutSummary(stderr string) string {
	prefix := "[bridge] " + ArtifactTimeoutMarker
	summary := ""
	for _, rawLine := range strings.Split(stderr, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, prefix) {
			summary = boundSummary(strings.TrimPrefix(line, "[bridge] "))
		}
	}
	return summary
}

func boundSummary(line string) string {
	runes := []rune(line)
	if len(runes) <= maxSummaryRunes {
		return line
	}
	return string(runes[:maxSummaryRunes-1]) + "…"
}

func timeoutCauseCode(stderr string) string {
	summary := ArtifactTimeoutSummary(stderr)
	prefix := ArtifactTimeoutMarker + "cause="
	if !strings.HasPrefix(summary, prefix) {
		return ""
	}
	fields := strings.Fields(strings.TrimPrefix(summary, prefix))
	if len(fields) == 0 {
		return ""
	}
	if value := TimeoutCause(fields[0]); value.Known() {
		return string(value)
	}
	return ""
}

const escalationMarker = "escalation report written (pattern="

const exhaustedMarker = "EXHAUSTED: pane shows a quota/rate-limit wall"

var exhaustedLineRE = regexp.MustCompile(`^\[[a-z0-9-]+\] ` + regexp.QuoteMeta(exhaustedMarker))

func escalationLine(stderr string) string {
	for _, rawLine := range strings.Split(stderr, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "[auto-respond] "+escalationMarker) && strings.HasSuffix(line, "reason=escalate)"):
			return boundCause(strings.TrimPrefix(line, "[auto-respond] "))
		case exhaustedLineRE.MatchString(line):
			return boundCause(line[strings.Index(line, exhaustedMarker):])
		}
	}
	return ""
}

func escalationCause(stderr string) string {
	line := escalationLine(stderr)
	if strings.HasPrefix(line, exhaustedMarker) {
		return "rate_limit"
	}
	_, rest, found := strings.Cut(line, escalationMarker)
	if !found {
		return ""
	}
	pattern, _, _ := strings.Cut(rest, " ")
	if pattern == "" || pattern == "unknown" {
		return ""
	}
	return pattern
}
