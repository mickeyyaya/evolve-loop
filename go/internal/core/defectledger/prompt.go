package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// PromptBlock renders the continuation-disposition duty into the audit
// dispatch prompt: the ancestor's OPEN ids + texts and the artifact they are
// owed in — composed from the SAME records the gate grades against (workspace
// manifest, registry binding fallback, ancestor ledger). It is the half of
// "continuations must be TOLD their inherited defects" the auditor owns. The
// prompt is context, never enforcement: on a read fault it degrades to "" and
// says so with one INFO (AUDIT_LEDGER_PROMPT_DEGRADED) — the gate blocks the
// same fault loudly at Classify; absence and a non-continuation are silent.
func (l *Ledger) PromptBlock(req Request) string {
	if req.Workspace == "" || req.ProjectRoot == "" {
		return ""
	}
	cont, isCont, err := continuation.ReadManifest(req.Workspace)
	if err != nil || !isCont {
		reg, has := l.laneRegistryBinding(req)
		if has {
			cont, isCont = reg, true // manifest-less registry binding is still owed dispositions
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

// promptRows renders one "- id: text" line per OPEN row. The ledger text is
// AGENT-authored (a prior cycle's verdict sentinel), so it is rendered
// single-line and capped: an embedded "\n## …" can never masquerade as
// mechanism-authored prompt structure beside the MANDATORY heading.
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
