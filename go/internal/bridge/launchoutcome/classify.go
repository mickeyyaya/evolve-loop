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

type Attempt struct {
	Code    int
	CtxErr  error
	Stderr  string
	Refusal Refusal
}

func Classify(code int, ctxErr error, stderr string) Outcome {
	return Attempt{Code: code, CtxErr: ctxErr, Stderr: stderr}.Classify()
}

func (a Attempt) Classify() Outcome {
	code, ctxErr, stderr := a.Code, a.CtxErr, a.Stderr
	if code == ExitOK {
		return Outcome{}
	}
	row := classOf(code)
	out := Outcome{ExitCode: code, CauseCode: causeCodeOf(row, stderr, a.Refusal), CtxCancelled: ctxErr != nil, Signal: row.signal}
	if row.code == ExitUnknownPrompt && quotaCauses[out.CauseCode] {
		out.Signal = CodeExitQuotaWall
	}
	msg := fmt.Sprintf("bridge: launch exit=%d", code)
	if detail := a.detail(out.CauseCode); detail != "" {
		msg += ": " + detail
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

var quotaCauses = map[string]bool{"rate_limit": true, "exhausted": true}

func (a Attempt) detail(cause string) string {
	if cause == core.CauseGuardRefused {
		return a.Refusal.Text()
	}
	return causeLine(a.Code, a.Stderr)
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

func causeCodeOf(row exitClass, stderr string, refusal Refusal) string {
	if row.code == ExitArtifactTimeout {
		cause := timeoutCauseCode(stderr)
		if refusal.Present() && (cause == "" || cause == string(TimeoutIncomplete)) {
			return core.CauseGuardRefused
		}
		if cause != "" {
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
	return Attempt{Code: code, Stderr: stderr}.CauseCode()
}

func (a Attempt) CauseCode() string {
	if a.Code == ExitOK {
		return ""
	}
	return causeCodeOf(classOf(a.Code), a.Stderr, a.Refusal)
}
