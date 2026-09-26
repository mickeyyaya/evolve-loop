package launchoutcome

import (
	"regexp"
	"strings"
)

// ArtifactTimeoutMarker prefixes the ONE self-describing artifact-wait summary
// line the tmux wait diagnostic writes and the classifier parses. Hyphenated +
// colon-space so it can never collide with the StopArtifactTimeout kind string
// ("artifact_timeout"); bridge projects it as its artifactTimeoutMarker.
const ArtifactTimeoutMarker = "artifact-timeout: "

// TimeoutCause is the artifact-timeout sub-cause vocabulary: what the tmux
// wait diagnostic emits after `cause=` and the only tokens the ledger
// projection admits (bridge keeps a type alias + seven const aliases, so the
// emitter and the parser read one closed set).
type TimeoutCause string

// The seven-token vocabulary (driver_tmux_wait_diagnostic.go's selection order).
const (
	TimeoutContextCancelled  TimeoutCause = "context_cancelled"
	TimeoutDetectorError     TimeoutCause = "completion_detector_error"
	TimeoutSubmitWedged      TimeoutCause = "submit_wedged"
	TimeoutTransientUpstream TimeoutCause = "transient_upstream"
	TimeoutReviewStop        TimeoutCause = "review_stop"
	TimeoutReviewPause       TimeoutCause = "review_pause"
	TimeoutIncomplete        TimeoutCause = "incomplete"
)

// Known reports membership in the seven-token vocabulary — free-form `cause=`
// prose never becomes a typed cause.
func (c TimeoutCause) Known() bool {
	switch c {
	case TimeoutContextCancelled, TimeoutDetectorError, TimeoutSubmitWedged,
		TimeoutTransientUpstream, TimeoutReviewStop, TimeoutReviewPause,
		TimeoutIncomplete:
		return true
	}
	return false
}

const (
	// maxCauseRunes bounds the cause line threaded into the error chain.
	maxCauseRunes = 300
	// maxSummaryRunes is the artifact-timeout summary's own, larger budget:
	// the marker line carries waited / extends / the last review verdict.
	maxSummaryRunes = 1024
)

// firstDiagnosticLine picks the one-line cause threaded into the launch
// error chain. Validate-gauntlet failures put the cause FIRST and prefix it
// "[bridge]"; driver failures accumulate launch chatter first and end with
// the causal line (cycle-286: a timeout's first line was a stream_output
// NOTE while "FAIL: completion never signalled" sat last). So: the first
// "[bridge]"-prefixed line wins; otherwise the LAST non-empty line.
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

// boundCause caps the cause line rune-safely (never split UTF-8 mid-sequence).
func boundCause(line string) string {
	if runes := []rune(line); len(runes) > maxCauseRunes {
		return string(runes[:maxCauseRunes]) + "…"
	}
	return line
}

// ArtifactTimeoutSummary lifts the artifact wait's self-describing summary
// line out of a launch's stderr. The exact [bridge] prefix identifies marker
// candidates rather than inline evidence. The final candidate wins because
// earlier free-form diagnostics are sanitized and closeout emits the
// authoritative marker last. Returns "" when the driver produced no summary
// (e.g. a non-tmux driver returning 81), leaving the legacy cause in place.
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

// boundSummary caps the summary at its dedicated budget, "…" replacing the
// last kept rune.
func boundSummary(line string) string {
	runes := []rune(line)
	if len(runes) <= maxSummaryRunes {
		return line
	}
	return string(runes[:maxSummaryRunes-1]) + "…"
}

// timeoutCauseCode returns the Known() sub-cause the marker line names after
// `cause=`, or "" — the ledger's typed cause never comes from prose.
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

// escalationMarker is the auto-responder's report line; its pattern names what the pane showed.
const escalationMarker = "escalation report written (pattern="

// exhaustedMarker is the checkpoint's corroborated quota wall, which also exits 85; the driver tag precedes it.
const exhaustedMarker = "EXHAUSTED: pane shows a quota/rate-limit wall"

var exhaustedLineRE = regexp.MustCompile(`^\[[a-z0-9-]+\] ` + regexp.QuoteMeta(exhaustedMarker))

// escalationLine is the stderr line that explains an exit 85: the escalation report, or the corroborated
// wall; "" when neither was written (the loop guard's report is not an escalation).
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

// escalationCause is the ledger sub-cause of an exit 85: the pattern the auto-responder matched (rate_limit,
// model_unsupported, …), rate_limit for a corroborated wall, and "" for a prompt nobody recognised.
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
