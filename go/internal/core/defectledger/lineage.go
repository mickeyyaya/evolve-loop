package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func (l *Ledger) arm(req Request) (continuation.Continuation, bool, Verdict) {
	if req.Workspace == "" || req.ProjectRoot == "" {
		return continuation.Continuation{}, false, Verdict{}
	}
	cont, isContinuation, err := continuation.ReadManifest(req.Workspace)
	if err != nil {
		v := blockedOn(fmt.Sprintf("defect ledger: continuation manifest is unreadable (%s) — a continuation cannot be graded against a lineage it cannot read, and degrading open here is the gate's cheapest bypass", err.Error()))
		l.emit("Ledger.Reconcile", req, CodeManifestUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "arm", "blocked": "true", "workspace": req.Workspace})
		return continuation.Continuation{}, false, v
	}
	registryCont, hasRegistry := l.laneRegistryBinding(req)
	switch {
	case !isContinuation && !hasRegistry:
		return continuation.Continuation{}, false, Verdict{}
	case !isContinuation && hasRegistry:
		registryPath := continuation.RegistryPath(req.ProjectRoot)
		v := blockedOn(fmt.Sprintf("defect ledger: this workspace holds no continuation manifest, but the root-owned %s binds this lane's scope to cycle-%d — the manifest was deleted or never written. Inherited defects are reconciled from the registry binding; the missing manifest is itself the finding.", registryPath, registryCont.Cycle))
		l.emit("Ledger.Reconcile", req, CodeManifestMissing, v.Diagnostics[0].Message, map[string]string{"step": "arm", "blocked": "true", "registry_path": registryPath, "ancestor_cycle": strconv.Itoa(registryCont.Cycle)})
		return registryCont, true, v
	case hasRegistry && registryCont.Cycle != cont.Cycle:
		v := blockedOn(fmt.Sprintf("defect ledger: the workspace continuation manifest names cycle-%d but the root-owned registry binds this lane to cycle-%d — a rewritten manifest would re-point the gate at an ancestor with no open defects; resolve the disagreement before this cycle can PASS", cont.Cycle, registryCont.Cycle))
		l.emit("Ledger.Reconcile", req, CodeLineageDisagrees, v.Diagnostics[0].Message, map[string]string{"step": "arm", "blocked": "true", "manifest_cycle": strconv.Itoa(cont.Cycle), "registry_cycle": strconv.Itoa(registryCont.Cycle)})
		return continuation.Continuation{}, false, v
	}
	return cont, true, Verdict{}
}

func (l *Ledger) laneRegistryBinding(req Request) (continuation.Continuation, bool) {
	for _, id := range l.laneScope(req.Workspace) {
		c, ok, rerr := continuation.ReadRegistryEntry(req.ProjectRoot, id)
		if rerr == nil && ok {
			return c, true
		}
	}
	return continuation.Continuation{}, false
}

type lineage struct {
	ancestor, current Doc
}

func (l *Ledger) loadLineage(req Request, cont continuation.Continuation) (lineage, Verdict, bool) {
	ancestorWS := paths.RunWorkspace(req.ProjectRoot, cont.Cycle)
	ancestorPath := filepath.Join(ancestorWS, LedgerFile)
	ancestor, hasLedger, fault := read(ancestorWS)
	if fault != nil {
		v := blockedOn(fmt.Sprintf("defect ledger: ancestor cycle-%d ledger is unreadable (%s) — a continuation cannot be graded against a ledger it cannot read", cont.Cycle, fault.Error()))
		l.emit("Ledger.Reconcile", req, CodeLedgerUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "grade", "blocked": "true", "which": "ancestor", "op": fault.op, "path": ancestorPath, "ancestor_cycle": strconv.Itoa(cont.Cycle)})
		return lineage{}, v, false
	}
	if !hasLedger || !hasOwedRow(ancestor) {
		msg := fmt.Sprintf("defect ledger: this cycle continues cycle-%d, which left no reconcilable %s in %s — NO inherited defect is being enforced here. Expected for an ancestor that predates the ledger; a deleted ledger looks identical, so it is recorded rather than assumed benign.", cont.Cycle, LedgerFile, ancestorWS)
		l.emit("Ledger.Reconcile", req, CodeAncestorEmpty, msg, map[string]string{"step": "grade", "blocked": "false", "ancestor_cycle": strconv.Itoa(cont.Cycle), "path": ancestorPath})
		return lineage{ancestor: ancestor}, Verdict{Diagnostics: []cyclestate.Diagnostic{warningDiag(msg)}}, false
	}
	current, _, cfault := read(req.Workspace)
	if cfault != nil {
		v := blockedOn(fmt.Sprintf("defect ledger: this cycle's own %s is unreadable (%s) — reconciling would overwrite a record that cannot be read", LedgerFile, cfault.Error()))
		l.emit("Ledger.Reconcile", req, CodeLedgerUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "grade", "blocked": "true", "which": "own", "op": cfault.op, "path": filepath.Join(req.Workspace, LedgerFile), "ancestor_cycle": strconv.Itoa(cont.Cycle)})
		return lineage{}, v, false
	}
	return lineage{ancestor: ancestor, current: current}, Verdict{}, true
}

func vouchedCycles(cont continuation.Continuation, ancestor Doc) []int {
	if !hasOwedRow(ancestor) {
		return nil
	}
	cycles := []int{cont.Cycle}
	if ancestor.OriginCycle > 0 && ancestor.OriginCycle != cont.Cycle {
		cycles = append(cycles, ancestor.OriginCycle)
	}
	return cycles
}

func hasOwedRow(doc Doc) bool {
	for _, e := range doc.Entries {
		if e.Source == "" || e.Status == StatusOpen {
			return true
		}
	}
	return false
}
