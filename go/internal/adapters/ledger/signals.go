package ledger

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeLedgerAppendFailed signalcenter.Code = "LEDGER_APPEND_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLedger, CodeLedgerAppendFailed, "the ledger could not append an entry (lock, chain or I/O failure); the reason is the error, fields name the entry")
}

func WithSignals(signals *signalcenter.Center) Option {
	return func(l *FileLedger) {
		if signals == nil {
			return
		}
		l.onAppend = func(e core.LedgerEntry, err error) { signals.Emit(appendedEvent(e, err)) }
	}
}

func (l *FileLedger) SignalsWired() bool { return l.onAppend != nil }

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
