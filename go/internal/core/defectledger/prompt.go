package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func (l *Ledger) PromptBlock(req Request) string {
	if req.Workspace == "" || req.ProjectRoot == "" {
		return ""
	}
	cont, isCont, err := continuation.ReadManifest(req.Workspace)
	if err != nil || !isCont {
		reg, has := l.laneRegistryBinding(req)
		if has {
			cont, isCont = reg, true
		}
		if err != nil {
			fallback := "none"
			if has {
				fallback = "registry"
			}
			l.emit("Ledger.PromptBlock", req, CodePromptDegraded, "defect ledger: continuation manifest is unreadable while composing the audit prompt ("+err.Error()+"); fallback: "+fallback,
				map[string]string{"step": "prompt", "blocked": "false", "reason": "manifest", "fallback": fallback, "path": filepath.Join(req.Workspace, continuation.ManifestName)})
		}
	}
	if !isCont {
		return ""
	}
	ancestorWS := paths.RunWorkspace(req.ProjectRoot, cont.Cycle)
	doc, hasLedger, fault := read(ancestorWS)
	if fault != nil {
		l.emit("Ledger.PromptBlock", req, CodePromptDegraded, "defect ledger: ancestor cycle-"+strconv.Itoa(cont.Cycle)+" ledger is unreadable while composing the audit prompt ("+fault.Error()+")",
			map[string]string{"step": "prompt", "blocked": "false", "reason": "ledger", "op": fault.op, "ancestor_cycle": strconv.Itoa(cont.Cycle), "path": filepath.Join(ancestorWS, LedgerFile)})
		return ""
	}
	if !hasLedger {
		return ""
	}
	rows := promptRows(doc.OpenEntries())
	if rows == "" {
		return ""
	}
	return fmt.Sprintf("\n## Inherited defect dispositions (MANDATORY)\n"+
		"This cycle continues cycle-%d. Write <workspace>/%s BEFORE emitting your verdict, one entry per id below — status FIXED (evidence: a bare resolving cite) or DEFERRED (a non-empty reason). Ids are copied verbatim, never renumbered.\n%s",
		cont.Cycle, DispositionsFile, rows)
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
		fmt.Fprintf(&ids, "- %s: %s\n", e.ID, text)
	}
	return ids.String()
}
