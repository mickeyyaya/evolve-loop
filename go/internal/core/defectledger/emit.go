package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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
	doc, added, overflow := Append(doc, rejectionRows(r), req.Cycle)
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

func (l *Ledger) emitFailed(req Request, err error, op, path string) Verdict {
	msg := "defect ledger: could not record this cycle's defects (" + err.Error() + ") — a later continuation will have nothing to reconcile against"
	l.emit("Ledger.Emit", req, CodeEmitFailed, msg, map[string]string{"step": "emit", "blocked": "false", "op": op, "path": path})
	return Verdict{Diagnostics: []cyclestate.Diagnostic{warningDiag(msg)}}
}

func rejectionRows(r Rejection) []Entry {
	rows := make([]Entry, 0, len(r.Defects)+len(r.Prescriptions))
	for _, d := range r.Defects {
		rows = append(rows, Entry{Text: d, Status: StatusOpen, Severity: severityOf(d, r.Findings)})
	}
	for _, p := range r.Prescriptions {
		rows = append(rows, Entry{Text: PrescriptionPrefix + p, Status: StatusOpen})
	}
	return rows
}

const overflowClause = "were not recorded: the ledger cap of"

func overflowRow(overflow, cycle int) Entry {
	text := fmt.Sprintf("%d further defect(s) from cycle-%d %s %d entries was reached", overflow, cycle, overflowClause, MaxEntries)
	return Entry{ID: ID(text), Text: text, Status: StatusOpen}
}
