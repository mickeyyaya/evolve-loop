package failurediag

import (
	"strconv"
	"strings"
)

const (
	MarkerPrefix            = "artifact-timeout: "
	CauseField              = "cause="
	ReasonField             = "reason="
	CauseSubmitWedged       = "submit_wedged"
	ExitCodeArtifactTimeout = 81
)

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
