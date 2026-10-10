package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func (l *Ledger) Reconcile(req Request) Verdict {
	cont, armed, v := l.arm(req)
	if !armed {
		return v
	}
	graded, ancestor := l.grade(req, cont)
	v.Diagnostics = append(v.Diagnostics, graded.Diagnostics...)
	v.Blocked = v.Blocked || graded.Blocked
	if v.Blocked {
		return v
	}
	v.LineageCycles = vouchedCycles(cont, ancestor)
	return v
}

func (l *Ledger) grade(req Request, cont continuation.Continuation) (Verdict, Doc) {
	lin, v, ok := l.loadLineage(req, cont)
	if !ok {
		return v, lin.ancestor
	}
	claims, diags, blocked := l.ReadDispositions(req, cont.Cycle)
	if blocked {
		return Verdict{Diagnostics: diags, Blocked: true}, lin.ancestor
	}
	diags = append(diags, l.Preflight(req, cont.Cycle, lin.ancestor.Entries, claims)...)
	resolve := func(evidence string) (bool, string) { return l.resolve(evidence, req) }
	merged, missing := mergeInherited(lin.current.Entries, lin.ancestor.Entries, claims, resolve)
	doc := Doc{OriginCycle: originCycleOf(lin.ancestor, lin.current), Entries: merged}
	return l.writeBack(req, cont, doc, missing, diags), lin.ancestor
}

func (l *Ledger) writeBack(req Request, cont continuation.Continuation, doc Doc, missing []unaccounted, diags []cyclestate.Diagnostic) Verdict {
	if err := Write(req.Workspace, doc); err != nil {
		msg := fmt.Sprintf("defect ledger: could not write back the reconciled ledger (%s) — an invisible disposition is not a disposition", err.Error())
		l.emit("Ledger.Reconcile", req, CodeWritebackFailed, msg, map[string]string{"step": "grade", "blocked": "true", "path": filepath.Join(req.Workspace, LedgerFile)})
		return Verdict{Diagnostics: append(diags, errorDiag(msg)), Blocked: true}
	}
	if len(missing) == 0 {
		return Verdict{Diagnostics: diags}
	}
	rendered, ids := make([]string, len(missing)), make([]string, len(missing))
	for i, u := range missing {
		rendered[i], ids[i] = u.String(), u.id
	}
	head := fmt.Sprintf("defect ledger: %d defect(s) inherited from cycle-%d are unaccounted for", len(missing), cont.Cycle)
	msg := fmt.Sprintf("%s [%s] — a continuation may not PASS while an ancestor defect is neither FIXED (with evidence) nor DEFERRED (with a reason). Disposition each id in %s.", head, strings.Join(rendered, ", "), DispositionsFile)
	bounded, truncated := boundedIDs(ids)
	l.emit("Ledger.Reconcile", req, CodeDefectsUnaccounted, head+" — disposition each id in "+DispositionsFile, map[string]string{
		"step": "grade", "blocked": "true", "ancestor_cycle": strconv.Itoa(cont.Cycle), "count": strconv.Itoa(len(missing)),
		"ids": bounded, "ids_truncated": truncated, "path": filepath.Join(req.Workspace, DispositionsFile),
	})
	return Verdict{Diagnostics: append(diags, errorDiag(msg)), Blocked: true}
}

type unaccounted struct {
	id, why string
}

func (u unaccounted) String() string { return u.id + " (" + u.why + ")" }

func mergeInherited(current, ancestor []Entry, claims map[string]Entry, resolve func(string) (bool, string)) ([]Entry, []unaccounted) {
	merged := append([]Entry(nil), current...)
	pos := make(map[string]int, len(merged)+len(ancestor))
	for i, e := range merged {
		if _, dup := pos[e.ID]; dup {
			continue
		}
		pos[e.ID] = i
	}
	var missing []unaccounted
	for _, a := range ancestor {
		i, carried := pos[a.ID]
		if !carried {
			i = len(merged)
			pos[a.ID] = i
			merged = append(merged, a)
		} else if merged[i].Text != a.Text {
			missing = append(missing, unaccounted{a.ID, fmt.Sprintf("id shadowed: this cycle's ledger holds different text %q for the same id", Truncate(merged[i].Text, 120))})
			merged[i] = reopened(a)
			continue
		}
		if a.Status != StatusOpen {
			merged[i] = a
			continue
		}
		claim, has := claims[a.ID]
		e, why := gradeClaim(a, claim, has, resolve)
		if why != "" {
			missing = append(missing, unaccounted{a.ID, why})
		}
		merged[i] = e
	}
	return merged, missing
}

func gradeClaim(a Entry, claim Entry, has bool, resolve func(string) (bool, string)) (Entry, string) {
	e := reopened(a)
	switch {
	case !has:
		return e, "no disposition"
	case claim.Status == StatusFixed:
		if ok, why := resolve(claim.Evidence); !ok {
			return e, "FIXED but " + why
		}
		e.Status, e.Evidence, e.Reason = claim.Status, claim.Evidence, claim.Reason
		return e, ""
	case claim.Status == StatusDeferred:
		if strings.TrimSpace(claim.Reason) == "" {
			return e, "DEFERRED without reason"
		}
		e.Status, e.Evidence, e.Reason = claim.Status, claim.Evidence, claim.Reason
		return e, ""
	default:
		return e, fmt.Sprintf("status %q is not FIXED or DEFERRED", claim.Status)
	}
}

func originCycleOf(ancestor, current Doc) int {
	if ancestor.OriginCycle == 0 {
		return current.OriginCycle
	}
	return ancestor.OriginCycle
}

func reopened(a Entry) Entry {
	return Entry{ID: a.ID, Text: a.Text, Status: StatusOpen, Source: a.Source, Round: a.Round, Severity: a.Severity, Dimension: a.Dimension}
}
