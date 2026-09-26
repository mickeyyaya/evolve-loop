package launchoutcome

import (
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Outcome is what one launch exit MEANS — the value object the error chain,
// the attempt ledger and the signal stream all read from. The zero value is
// ExitOK: no error, no cause, no code.
type Outcome struct {
	// ExitCode is the exit as the driver returned it.
	ExitCode int
	// CauseCode is the attempt ledger's cause_code: the exit's class name, or
	// on 81 the marker's Known() sub-cause; driver_error for a signal death
	// and for any exit the table does not name (the ledger never changes).
	CauseCode string
	// Transient reports the exits the orchestrator retries or reconciles
	// instead of hard-failing: 80 / 85 / 86 / 124, or a signal death under a
	// cancelled context.
	Transient bool
	// CtxCancelled records ctx.Err() != nil at classification — a field for
	// the triage; only the signal-death row acts on it.
	CtxCancelled bool
	// Signal is the registered BRIDGE_EXIT_* code — the table's signal column,
	// spelled by class name (81 is ONE code whatever the sub-cause; the
	// sub-cause rides CauseCode); "" on ExitOK — success carries no code.
	Signal signalcenter.Code
	// Err is the error Launch returns: "bridge: launch exit=<code>[: <cause>]",
	// wrapped "%s: %w" with core.ErrArtifactTimeout (81) or
	// core.ErrTransientBridgeFailure (Transient), plain errors.New otherwise.
	// retry_backoff.go byte-parses the prefix; the breaker fingerprints the
	// whole string.
	Err error
}

// Classify is pure: (exit code, ctx.Err() at return, the launch's captured
// stderr) → Outcome. No clock, no filesystem, no Center.
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

// causeLine is the bounded cause threaded into the error: the first
// diagnostic line, or on 81 the driver's marker summary when present — an
// artifact-timeout death must be self-describing (waited / extends consumed /
// the last review verdict), and for the tmux drivers the positional
// heuristic used to land on a workspace file listing. Scoped to 81 so no
// other exit's cause changes.
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

// causeCodeOf is the ledger column: a Known() 81 sub-cause from the marker
// line, else the row's cause code.
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

// CauseCode is the attempt ledger's ONE projection (llm-calls.ndjson
// cause_code): "" on ExitOK, else the table's cause column with the 81
// sub-cause rule.
func CauseCode(code int, stderr string) string {
	if code == ExitOK {
		return ""
	}
	return causeCodeOf(classOf(code), stderr)
}
