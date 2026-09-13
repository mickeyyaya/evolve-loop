package failurediag

import (
	"strconv"
	"strings"
)

// The artifact-timeout wire contract as the decoder spells it. The producer
// (internal/bridge: stopreview.go, driver_tmux_wait_diagnostic.go,
// attempt_telemetry.go) spells the same tokens; follow-up F2 re-points it at
// these constants so the two cannot drift.
const (
	// MarkerPrefix opens the bridge's artifact-timeout marker in a launch error.
	MarkerPrefix = "artifact-timeout: "
	// CauseField introduces the host-owned cause token that follows the prefix.
	CauseField = "cause="
	// ReasonField introduces the quoted reason that follows the cause token.
	ReasonField = "reason="
	// CauseSubmitWedged is the one cause that is an evidenced delivery failure.
	CauseSubmitWedged = "submit_wedged"
	// ExitCodeArtifactTimeout is the exit-code class the sidecar records for the gate.
	ExitCodeArtifactTimeout = 81
)

// DeliveryFailureCause classifies an evidenced prompt-delivery failure, or
// returns "" when err is not one. isTimeout is the orchestrator's
// artifact-timeout gate (core's ErrArtifactTimeout, injected — the unit never
// imports the sentinel): nothing is classified unless it says so, so reviewer
// prose cannot forge a delivery failure. With a host-owned cause= token the
// token is authoritative: any cause but submit_wedged is "", submit_wedged
// returns the quoted reason when it names the token and the bare cause
// otherwise, and reason= is honoured only IMMEDIATELY after the cause token.
// Without cause= (the legacy marker) the quoted reason= classifies only when
// it names submit_wedged. Malformed or truncated markers classify as nothing.
func DeliveryFailureCause(err error, isTimeout func(error) bool) string {
	if !isTimeout(err) {
		return ""
	}
	message := err.Error()
	cause, reason, hasCause := typedCause(message)
	if hasCause {
		if cause != CauseSubmitWedged {
			return ""
		}
		if strings.Contains(reason, CauseSubmitWedged) {
			return reason
		}
		return cause
	}
	_, marker, ok := strings.Cut(message, MarkerPrefix)
	if !ok {
		return ""
	}
	_, reasonField, ok := strings.Cut(marker, ReasonField)
	if !ok {
		return ""
	}
	reason, ok = quotedValue(reasonField)
	if ok && strings.Contains(reason, CauseSubmitWedged) {
		return reason
	}
	return ""
}

// typedCause reads the host-owned "cause=<token> reason=<quoted>" marker;
// present is false for the legacy marker without a cause token.
func typedCause(message string) (cause, reason string, present bool) {
	_, marker, ok := strings.Cut(message, MarkerPrefix)
	if !ok || !strings.HasPrefix(marker, CauseField) {
		return "", "", false
	}
	causeField, rest, found := strings.Cut(strings.TrimPrefix(marker, CauseField), " ")
	if !found {
		return causeField, "", true
	}
	if !strings.HasPrefix(rest, ReasonField) {
		return causeField, "", true
	}
	reason, _ = quotedValue(strings.TrimPrefix(rest, ReasonField))
	return causeField, reason, true
}

// quotedValue decodes a leading Go-quoted string, walking escapes; false when
// the value is not quoted or the quote is never terminated.
func quotedValue(value string) (string, bool) {
	if value == "" || value[0] != '"' {
		return "", false
	}
	for i := 1; i < len(value); i++ {
		switch value[i] {
		case '\\':
			i++
		case '"':
			decoded, err := strconv.Unquote(value[:i+1])
			return decoded, err == nil
		}
	}
	return "", false
}
