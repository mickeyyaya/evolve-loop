package launchoutcome

import (
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Outcome struct {
	ExitCode     int
	CauseCode    string
	Transient    bool
	CtxCancelled bool
	Signal       signalcenter.Code
	Err          error
}

func Classify(code int, ctxErr error, stderr string) Outcome {
	if code == ExitOK {
		return Outcome{}
	}
	row := classOf(code)
	out := Outcome{ExitCode: code, CauseCode: causeCodeOf(row, stderr), CtxCancelled: ctxErr != nil, Signal: row.signal}
	msg := fmt.Sprintf("bridge: launch exit=%d", code)
	if cause := causeLine(code, stderr); cause != "" {
		msg += ": " + cause
	}
	sentinel := row.sentinel
	if row.sentinelOnCancel && ctxErr == nil {
		sentinel = nil
	}
	out.Transient = errors.Is(sentinel, core.ErrTransientBridgeFailure)
	if sentinel == nil {
		out.Err = errors.New(msg)
		return out
	}
	out.Err = fmt.Errorf("%s: %w", msg, sentinel)
	return out
}

func causeLine(code int, stderr string) string {
	cause := firstDiagnosticLine(stderr)
	if code == ExitArtifactTimeout {
		if summary := ArtifactTimeoutSummary(stderr); summary != "" {
			cause = summary
		}
	}
	if code == ExitUnknownPrompt {
		if line := escalationLine(stderr); line != "" {
			cause = line
		}
	}
	return cause
}

func causeCodeOf(row exitClass, stderr string) string {
	if row.code == ExitArtifactTimeout {
		if cause := timeoutCauseCode(stderr); cause != "" {
			return cause
		}
	}
	if row.code == ExitUnknownPrompt {
		if cause := escalationCause(stderr); cause != "" {
			return cause
		}
	}
	return row.causeCode
}

func CauseCode(code int, stderr string) string {
	if code == ExitOK {
		return ""
	}
	return causeCodeOf(classOf(code), stderr)
}
