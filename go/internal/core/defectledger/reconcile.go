package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Reconcile is the disposition diff: it returns the diagnostics to surface,
// whether the cycle must be blocked from PASS, and the lineage a clean grade
// vouches for. Blocking cases are exactly the ones where an inherited defect
// could otherwise vanish: an unaccounted OPEN entry, an unevidenced closure
// claim, an unreadable manifest or ledger, or a write-back failure (a
// disposition that did not reach disk is not visible, which is the whole
// point). A cycle that is not a continuation is a clean no-op.
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

// grade is the disposition diff proper against an established lineage: load
// both ledgers, read the claims, pre-flight the artifact, merge the inherited
// rows with their graded status, write the merged ledger back BEFORE the
// verdict, then name what is unaccounted. It returns the ancestor Doc it
// loaded so the vouch needs no second read.
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

// writeBack lands the merged ledger BEFORE grading — the operator must be
// able to read what this cycle disposed of even on the run where the gate
// blocks — then blocks on anything unaccounted.
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

// unaccounted is one inherited defect the grade could not close, with why.
type unaccounted struct {
	id, why string
}

func (u unaccounted) String() string { return u.id + " (" + u.why + ")" }

// mergeInherited MERGES the ancestor's rows onto this workspace's ledger. The
// inherited rows are rebuilt from the ANCESTOR on every pass and their status
// derives ONLY from the claims, never from the row already in this workspace
// (cycle-1282 DEF-1: `current` is a file the graded agent may write, so a
// pre-planted FIXED row must satisfy nothing). What `current` contributes is
// this cycle's own emitted rows. Entries transition; they are never deleted.
func mergeInherited(current, ancestor []Entry, claims map[string]Entry, resolve func(string) (bool, string)) ([]Entry, []unaccounted) {
	merged := append([]Entry(nil), current...)
	pos := make(map[string]int, len(merged)+len(ancestor))
	for i, e := range merged {
		if _, dup := pos[e.ID]; dup {
			continue // FIRST row wins; a later duplicate must not become the index target
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
			// Same id, different text: a defectID collision or a planted row
			// aimed at an inherited id. Both are loud, and in both cases the
			// ANCESTOR's text is the record — a shadowed id means the operator
			// cannot trust any disposition keyed on it.
			missing = append(missing, unaccounted{a.ID, fmt.Sprintf("id shadowed: this cycle's ledger holds different text %q for the same id", Truncate(merged[i].Text, 120))})
			merged[i] = Entry{ID: a.ID, Text: a.Text, Status: StatusOpen}
			continue
		}
		if a.Status != StatusOpen {
			merged[i] = a // dispositioned upstream — carried verbatim, evidence and reason included
			continue
		}
		claim, has := claims[a.ID]
		e, why := gradeClaim(a, claim, has, resolve)
		if why != "" {
			missing = append(missing, unaccounted{a.ID, why})
		}
		merged[i] = e // an unaccounted entry stays OPEN — it is never dropped
	}
	return merged, missing
}

// gradeClaim is the status table for one inherited OPEN row, fresh from the
// ancestor and never from the workspace row: FIXED needs a resolving cite,
// DEFERRED a non-empty reason, anything else stays OPEN with the reason it
// is unaccounted. A rejected FIXED row is written back with no evidence or
// reason — an unverifiable FIXED row IS the laundering.
func gradeClaim(a Entry, claim Entry, has bool, resolve func(string) (bool, string)) (Entry, string) {
	e := Entry{ID: a.ID, Text: a.Text, Status: StatusOpen}
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

// originCycleOf is the written-back origin: the ancestor's, or this cycle's
// own when the ancestor carries none.
func originCycleOf(ancestor, current Doc) int {
	if ancestor.OriginCycle == 0 {
		return current.OriginCycle
	}
	return ancestor.OriginCycle
}
