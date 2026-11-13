package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type inheritance struct {
	cont        continuation.Continuation
	open        []Entry
	manifestErr error
	fallback    string
	ledgerFault *readFault
}

func (l *Ledger) inheritedOpen(req Request) inheritance {
	var in inheritance
	if req.Workspace == "" || req.ProjectRoot == "" {
		return in
	}
	cont, isCont, err := continuation.ReadManifest(req.Workspace)
	if err != nil || !isCont {
		reg, has := l.laneRegistryBinding(req)
		if has {
			cont, isCont = reg, true
		}
		if err != nil {
			in.manifestErr = err
			in.fallback = "none"
			if has {
				in.fallback = "registry"
			}
		}
	}
	if !isCont {
		return in
	}
	in.cont = cont
	doc, hasLedger, fault := read(paths.RunWorkspace(req.ProjectRoot, cont.Cycle))
	if fault != nil {
		in.ledgerFault = fault
		return in
	}
	if hasLedger {
		in.open = doc.OpenEntries()
	}
	return in
}

func (l *Ledger) DispositionsRequired(req Request) bool {
	return len(l.inheritedOpen(req).open) > 0
}

func (l *Ledger) PromptBlock(req Request) string {
	in := l.inheritedOpen(req)
	if in.manifestErr != nil {
		l.emit("Ledger.PromptBlock", req, CodePromptDegraded, "defect ledger: continuation manifest is unreadable while composing the audit prompt ("+in.manifestErr.Error()+"); fallback: "+in.fallback,
			map[string]string{"step": "prompt", "blocked": "false", "reason": "manifest", "fallback": in.fallback, "path": filepath.Join(req.Workspace, continuation.ManifestName)})
	}
	if in.ledgerFault != nil {
		ancestorWS := paths.RunWorkspace(req.ProjectRoot, in.cont.Cycle)
		l.emit("Ledger.PromptBlock", req, CodePromptDegraded, "defect ledger: ancestor cycle-"+strconv.Itoa(in.cont.Cycle)+" ledger is unreadable while composing the audit prompt ("+in.ledgerFault.Error()+")",
			map[string]string{"step": "prompt", "blocked": "false", "reason": "ledger", "op": in.ledgerFault.op, "ancestor_cycle": strconv.Itoa(in.cont.Cycle), "path": filepath.Join(ancestorWS, LedgerFile)})
		return ""
	}
	rows := promptRows(in.open)
	if rows == "" {
		return ""
	}
	return fmt.Sprintf("\n## Inherited defect dispositions (MANDATORY)\n"+
		"This cycle continues cycle-%d. Write <workspace>/%s BEFORE emitting your verdict, one entry per id below — status FIXED (evidence: a bare resolving cite) or DEFERRED (a non-empty reason). Ids are copied verbatim, never renumbered.\n%s",
		in.cont.Cycle, DispositionsFile, rows)
}

func promptRows(open []Entry) string {
	var ids strings.Builder
	for _, e := range open {
		text := strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' {
				return ' '
			}
			return r
		}, Truncate(e.Text, 200))
		if blocksDeferral(e) {
			fmt.Fprintf(&ids, "- %s (%s blocker: FIXED only): %s\n", e.ID, e.Severity, text)
			continue
		}
		fmt.Fprintf(&ids, "- %s: %s\n", e.ID, text)
	}
	return ids.String()
}
