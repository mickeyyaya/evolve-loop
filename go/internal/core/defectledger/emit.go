package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Emit records a rejecting audit's structured defects and prescriptions as
// addressable OPEN entries in <workspace>/defect-ledger.json. A rejection with
// no structured content mints nothing: an empty ledger on every cycle would
// make every later cycle look like a continuation and is the cheapest way to
// make the reconcile gate vacuous. New rows are APPENDED to any ledger already
// in the workspace (a continuation that inherits entries and then raises its
// own must keep both) — merge, never replace, because replacement is deletion
// by another name. The Result Object is a Verdict that never blocks (the
// ledger is a record, not a gate, at emit time): a read or write fault is ONE
// warning carrying the graded wire the audit host appends verbatim, reported
// as AUDIT_LEDGER_EMIT_FAILED with the same text; an overflow past the cap is
// one synthetic OPEN row and AUDIT_LEDGER_OVERFLOW.
func (l *Ledger) Emit(req Request, r Rejection) Verdict {
	if req.Workspace == "" || (len(r.Defects) == 0 && len(r.Prescriptions) == 0) {
		return Verdict{}
	}
	path := filepath.Join(req.Workspace, LedgerFile)
	doc, existed, fault := read(req.Workspace)
	if fault != nil {
		return l.emitFailed(req, fault, fault.op, path)
	}
	if !existed {
		doc.OriginCycle = req.Cycle
	}
	doc, added, overflow := Append(doc, openRows(rowTexts(r)), req.Cycle)
	if overflow > 0 {
		l.emit("Ledger.Emit", req, CodeOverflow, fmt.Sprintf("defect ledger: %d defect(s) from cycle-%d were not recorded: the ledger cap of %d entries was reached", overflow, req.Cycle, MaxEntries),
			map[string]string{"step": "emit", "blocked": "false", "overflow": strconv.Itoa(overflow), "cap": strconv.Itoa(MaxEntries), "path": path})
	}
	if !added {
		return Verdict{}
	}
	if err := Write(req.Workspace, doc); err != nil {
		return l.emitFailed(req, err, "write", path)
	}
	return Verdict{}
}

// emitFailed is the ONE author of the emit-failure wire: the warning the
// audit host surfaces unchanged (the verdict stands; a later continuation
// has nothing to reconcile against) and the signal whose Reason is that same
// text — the diagnostic and the event can never say different things.
func (l *Ledger) emitFailed(req Request, err error, op, path string) Verdict {
	msg := "defect ledger: could not record this cycle's defects (" + err.Error() + ") — a later continuation will have nothing to reconcile against"
	l.emit("Ledger.Emit", req, CodeEmitFailed, msg, map[string]string{"step": "emit", "blocked": "false", "op": op, "path": path})
	return Verdict{Diagnostics: []cyclestate.Diagnostic{warningDiag(msg)}}
}

// rowTexts lists the rows a rejection mints: the defects verbatim, then the
// prescriptions tagged distinguishably (F3, scout report Hypothesis 2).
func rowTexts(r Rejection) []string {
	texts := make([]string, 0, len(r.Defects)+len(r.Prescriptions))
	texts = append(texts, r.Defects...)
	for _, p := range r.Prescriptions {
		texts = append(texts, PrescriptionPrefix+p)
	}
	return texts
}

func openRows(texts []string) []Entry {
	rows := make([]Entry, len(texts))
	for i, text := range texts {
		rows[i] = Entry{Text: text, Status: StatusOpen}
	}
	return rows
}

func overflowRow(overflow, cycle int) Entry {
	text := fmt.Sprintf("%d further defect(s) from cycle-%d were not recorded: the ledger cap of %d entries was reached", overflow, cycle, MaxEntries)
	return Entry{ID: ID(text), Text: text, Status: StatusOpen}
}
