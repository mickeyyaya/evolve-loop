package ledger

// signals.go — ADR-0101 S4a: the Signal Center observes the file ledger at its
// ONE append chokepoint. WithSignals is a construction-time option (functional
// options; explicit DI at the root) that installs the observer: every
// core.LedgerEntry written through Append — the orchestrator's records, the
// bridge's stop_review, the inbox mover's lifecycle lines (AppendLifecycle),
// the seal's segment anchor — is also a ledger.appended INFO signal naming
// its ledger line (fields.entry_seq); a failed Append is a WARN
// LEDGER_APPEND_FAILED and the error still returns. Deliberately NOT a wrapper
// type: Go embedding promotes methods without virtual dispatch, so a Decorator
// over *FileLedger would let every promoted append path (AppendLifecycle,
// Seal) write unobserved — the S4a architecture review's HIGH-1. The lines the
// observer does not see (self-constructed ledgers, the repair marker) are the
// inventory pinned in signals_test.go.

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// CodeLedgerAppendFailed names a ledger append that returned an error.
const CodeLedgerAppendFailed signalcenter.Code = "LEDGER_APPEND_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLedger, CodeLedgerAppendFailed, "the ledger could not append an entry (lock, chain or I/O failure); the reason is the error, fields name the entry")
}

// WithSignals installs the Signal Center as the ledger's append observer. A
// nil Center installs nothing (the Null Object — tests only: the production
// root always passes its Center, and SignalsWired proves it).
func WithSignals(signals *signalcenter.Center) Option {
	return func(l *FileLedger) {
		if signals == nil {
			return
		}
		l.onAppend = func(e core.LedgerEntry, err error) { signals.Emit(appendedEvent(e, err)) }
	}
}

// SignalsWired reports whether the Signal Center observes this ledger — the
// root's wiring proof.
func (l *FileLedger) SignalsWired() bool { return l.onAppend != nil }

// appendedEvent projects one appended entry (or its failure) onto the Event
// (Adapter: the ledger's own record is the source; the signal names it).
func appendedEvent(e core.LedgerEntry, err error) signalcenter.Event {
	fields := map[string]string{"role": e.Role, "kind": e.Kind, "exit_code": strconv.Itoa(e.ExitCode)}
	if e.ArtifactPath != "" {
		fields["path"] = e.ArtifactPath
	}
	event := signalcenter.Event{
		Cycle: e.Cycle, Module: signalcenter.ModuleLedger, Origin: "FileLedger.Append", Kind: signalcenter.KindLedgerAppended,
		Severity: signalcenter.SeverityInfo, Reason: "ledger: " + e.Role + " " + e.Kind, Fields: fields,
	}
	if err != nil {
		event.Severity, event.Code, event.Reason = signalcenter.SeverityWarn, CodeLedgerAppendFailed, "ledger append failed: "+err.Error()
		return event
	}
	fields["entry_seq"] = strconv.Itoa(e.EntrySeq)
	return event
}
