package bridge

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type artifactTimeoutCause string

const (
	artifactTimeoutContextCancelled  artifactTimeoutCause = "context_cancelled"
	artifactTimeoutDetectorError     artifactTimeoutCause = "completion_detector_error"
	artifactTimeoutSubmitWedged      artifactTimeoutCause = "submit_wedged"
	artifactTimeoutTransientUpstream artifactTimeoutCause = "transient_upstream"
	artifactTimeoutReviewStop        artifactTimeoutCause = "review_stop"
	artifactTimeoutReviewPause       artifactTimeoutCause = "review_pause"
	artifactTimeoutIncomplete        artifactTimeoutCause = "incomplete"
)

type artifactTimeoutEvidence struct {
	cancellationErr         error
	terminalDetectorErrored bool
	submitWedged            bool
	transient               bool
	reviewAction            ReviewAction
}

func selectArtifactTimeoutCause(e artifactTimeoutEvidence) artifactTimeoutCause {
	switch {
	case e.cancellationErr != nil:
		return artifactTimeoutContextCancelled
	case e.terminalDetectorErrored:
		return artifactTimeoutDetectorError
	case e.submitWedged:
		return artifactTimeoutSubmitWedged
	case e.transient:
		return artifactTimeoutTransientUpstream
	case e.reviewAction == ReviewStop:
		return artifactTimeoutReviewStop
	case e.reviewAction == ReviewPause:
		return artifactTimeoutReviewPause
	default:
		return artifactTimeoutIncomplete
	}
}

type artifactTimeoutDiagnostic struct {
	cause         artifactTimeoutCause
	reason        string
	phase         string
	cycle         int
	driver        string
	artifact      string
	waitedS       int
	intervalS     int
	extendsUsed   int
	maxExtends    int
	lastReview    string
	liveness      string
	progressed    bool
	busy          bool
	transient     bool
	detectorError string
}

func newArtifactTimeoutDiagnostic(w replWaiter, state *replWaitState, transient bool) artifactTimeoutDiagnostic {
	evidence := artifactTimeoutEvidence{
		cancellationErr:         state.cancellationErr,
		terminalDetectorErrored: state.terminalDetectorErrored,
		submitWedged:            state.submitWedged,
		transient:               transient,
		reviewAction:            state.lastVerdict.Action,
	}
	reason := state.lastVerdict.Reason
	if state.cancellationErr != nil {
		reason = state.cancellationErr.Error()
	}
	detectorError := ""
	if state.lastDetectorErr != nil {
		detectorError = state.lastDetectorErr.Error()
	}
	return artifactTimeoutDiagnostic{
		cause:         selectArtifactTimeoutCause(evidence),
		reason:        reason,
		phase:         w.phaseName,
		cycle:         w.cfg.Cycle,
		driver:        w.launch.name,
		artifact:      filepath.Base(w.cfg.Artifact),
		waitedS:       state.waitedS,
		intervalS:     state.intervalS,
		extendsUsed:   state.attempt,
		maxExtends:    state.maxExtends,
		lastReview:    reviewActionOrNone(state.lastVerdict.Action),
		liveness:      livenessOrUnknown(state.lastEvent.State),
		progressed:    state.lastEvent.Progressed,
		busy:          state.lastEvent.Busy,
		transient:     transient,
		detectorError: detectorError,
	}
}

func (d artifactTimeoutDiagnostic) String() string {
	return fmt.Sprintf(
		"%scause=%s reason=%s phase=%s cycle=%d driver=%s artifact=%s "+
			"waited=%ds interval=%ds extends_used=%d max_extends=%d last_review=%s "+
			"liveness=%s progressed=%v busy=%v transient=%v detector_error=%s",
		artifactTimeoutMarker,
		d.cause,
		boundedDiagnosticQuote(d.reason, 220),
		boundedDiagnosticToken(d.phase, 48),
		d.cycle,
		boundedDiagnosticToken(d.driver, 48),
		boundedDiagnosticQuote(d.artifact, 96),
		d.waitedS,
		d.intervalS,
		d.extendsUsed,
		d.maxExtends,
		boundedDiagnosticToken(d.lastReview, 16),
		boundedDiagnosticToken(d.liveness, 24),
		d.progressed,
		d.busy,
		d.transient,
		boundedDiagnosticQuote(d.detectorError, 192),
	)
}

func boundedDiagnosticToken(value string, maxRunes int) string {
	value = strings.Map(func(r rune) rune {
		if isDiagnosticControl(r) || unicode.IsSpace(r) || r == '=' || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, value)
	if value == "" {
		value = "unknown"
	}
	return boundDiagnosticRunes(value, maxRunes)
}

func boundedDiagnosticQuote(value string, maxRunes int) string {
	clean := strings.Map(func(r rune) rune {
		if isDiagnosticControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, value)
	quoted := strconv.Quote(clean)
	if len([]rune(quoted)) <= maxRunes {
		return quoted
	}

	runes := []rune(clean)
	low, high := 0, len(runes)
	for low < high {
		mid := low + (high-low+1)/2
		candidate := strconv.Quote(boundedDiagnosticMiddle(runes, mid))
		if len([]rune(candidate)) <= maxRunes {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return strconv.Quote(boundedDiagnosticMiddle(runes, low))
}

func isDiagnosticControl(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

func boundedDiagnosticMiddle(runes []rune, retained int) string {
	head := (retained + 1) / 2
	tail := retained / 2
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

func boundDiagnosticRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-1]) + "…"
}

func (w replWaiter) writeArtifactTimeoutMarker(state *replWaitState, transient bool) {
	fmt.Fprintf(w.deps.Stderr, "[bridge] %s\n", newArtifactTimeoutDiagnostic(w, state, transient))
}
