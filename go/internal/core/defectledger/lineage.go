package defectledger

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// arm decides whether this cycle is a continuation and which record
// establishes that — ONE decision with one set of rules, separate from
// grading (the cycle-1285 F2 defect was entirely in the arming half while
// every defense lived in the grade). It returns the ancestor binding and true
// when the grade must run; otherwise the Verdict to return as-is (zero for a
// plain cycle, blocked for a corrupt manifest or a manifest/registry
// disagreement). A registry-only binding arms the grade AND blocks: the
// Verdict carries the missing-manifest finding for Reconcile to prepend.
func (l *Ledger) arm(req Request) (continuation.Continuation, bool, Verdict) {
	if req.Workspace == "" || req.ProjectRoot == "" {
		return continuation.Continuation{}, false, Verdict{}
	}
	cont, isContinuation, err := continuation.ReadManifest(req.Workspace)
	if err != nil {
		// cycle-1285 F2: one byte of garbage in a 0644 file in the graded
		// agent's own workspace used to retire every inherited defect. The
		// manifest decides whether the gate runs AT ALL, so it blocks, like an
		// unparseable defect-dispositions.json already did.
		v := blockedOn(fmt.Sprintf("defect ledger: continuation manifest is unreadable (%s) — a continuation cannot be graded against a lineage it cannot read, and degrading open here is the gate's cheapest bypass", err.Error()))
		l.emit("Ledger.Reconcile", req, CodeManifestUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "arm", "blocked": "true", "workspace": req.Workspace})
		return continuation.Continuation{}, false, v
	}
	// The deletion half of F2: arming may not depend SOLELY on a file inside
	// the workspace the graded agent writes. The lineage is also witnessed out
	// of band, in the root-owned continuation registry keyed by this lane's
	// scope, so the manifest is a workspace CACHE of a record that survives it.
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
		// Both records exist and disagree about the ancestor. The workspace
		// copy is the rewritable one, so it is the suspect; refusing to pick
		// is the only honest move.
		v := blockedOn(fmt.Sprintf("defect ledger: the workspace continuation manifest names cycle-%d but the root-owned registry binds this lane to cycle-%d — a rewritten manifest would re-point the gate at an ancestor with no open defects; resolve the disagreement before this cycle can PASS", cont.Cycle, registryCont.Cycle))
		l.emit("Ledger.Reconcile", req, CodeLineageDisagrees, v.Diagnostics[0].Message, map[string]string{"step": "arm", "blocked": "true", "manifest_cycle": strconv.Itoa(cont.Cycle), "registry_cycle": strconv.Itoa(registryCont.Cycle)})
		return continuation.Continuation{}, false, v
	}
	return cont, true, Verdict{}
}

// laneRegistryBinding returns this lane's continuation binding from the
// ROOT-OWNED registry, and whether one exists. The lane's identity is its
// pinned scope (the injected reader — absent, malformed or empty pin ⇒ nil ⇒
// no lineage); scoping the lookup to THIS lane's ids is what keeps the
// fallback from blocking ordinary cycles whose root registry legitimately
// carries other lanes' bindings. Fail-closed is not available here: an
// unreadable registry cannot manufacture a lineage, so a miss is a miss — the
// mechanism's known ceiling, recorded in continuation-defect-ledger.md.
func (l *Ledger) laneRegistryBinding(req Request) (continuation.Continuation, bool) {
	for _, id := range l.laneScope(req.Workspace) {
		c, ok, rerr := continuation.ReadRegistryEntry(req.ProjectRoot, id)
		if rerr == nil && ok {
			return c, true
		}
	}
	return continuation.Continuation{}, false
}

// lineage is what the grade reads: the ancestor's ledger and this cycle's own.
type lineage struct {
	ancestor, current Doc
}

// loadLineage reads the ancestor's ledger and this workspace's. An unreadable
// file on either side blocks (AUDIT_LEDGER_UNREADABLE, which = ancestor |
// own); an absent or empty ancestor ledger is a warning that enforces nothing
// (AUDIT_LEDGER_ANCESTOR_EMPTY) — legitimate when the ancestor predates the
// ledger, but a DELETED ancestor ledger is indistinguishable from that, so it
// is recorded rather than assumed benign. The third result says whether the
// grade proceeds.
func (l *Ledger) loadLineage(req Request, cont continuation.Continuation) (lineage, Verdict, bool) {
	ancestorWS := paths.RunWorkspace(req.ProjectRoot, cont.Cycle)
	ancestorPath := filepath.Join(ancestorWS, LedgerFile)
	ancestor, hasLedger, fault := read(ancestorWS)
	if fault != nil {
		v := blockedOn(fmt.Sprintf("defect ledger: ancestor cycle-%d ledger is unreadable (%s) — a continuation cannot be graded against a ledger it cannot read", cont.Cycle, fault.Error()))
		l.emit("Ledger.Reconcile", req, CodeLedgerUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "grade", "blocked": "true", "which": "ancestor", "op": fault.op, "path": ancestorPath, "ancestor_cycle": strconv.Itoa(cont.Cycle)})
		return lineage{}, v, false
	}
	if !hasLedger || len(ancestor.Entries) == 0 {
		msg := fmt.Sprintf("defect ledger: this cycle continues cycle-%d, which left no reconcilable %s in %s — NO inherited defect is being enforced here. Expected for an ancestor that predates the ledger; a deleted ledger looks identical, so it is recorded rather than assumed benign.", cont.Cycle, LedgerFile, ancestorWS)
		l.emit("Ledger.Reconcile", req, CodeAncestorEmpty, msg, map[string]string{"step": "grade", "blocked": "false", "ancestor_cycle": strconv.Itoa(cont.Cycle), "path": ancestorPath})
		return lineage{ancestor: ancestor}, Verdict{Diagnostics: []cyclestate.Diagnostic{warningDiag(msg)}}, false
	}
	// D1: reconcile MERGES onto the ledger already in this workspace.
	// Rebuilding from the ancestor alone and truncate-writing would erase the
	// rows Emit appended on a previous Classify in this same cycle.
	current, _, cfault := read(req.Workspace)
	if cfault != nil {
		v := blockedOn(fmt.Sprintf("defect ledger: this cycle's own %s is unreadable (%s) — reconciling would overwrite a record that cannot be read", LedgerFile, cfault.Error()))
		l.emit("Ledger.Reconcile", req, CodeLedgerUnreadable, v.Diagnostics[0].Message, map[string]string{"step": "grade", "blocked": "true", "which": "own", "op": cfault.op, "path": filepath.Join(req.Workspace, LedgerFile), "ancestor_cycle": strconv.Itoa(cont.Cycle)})
		return lineage{}, v, false
	}
	return lineage{ancestor: ancestor, current: current}, Verdict{}, true
}

// vouchedCycles is the lineage a graded, unblocked reconcile vouches for —
// the immediate ancestor plus the ledger's origin cycle — so the closure-
// citation gate's prose demand demotes to advisory for claims WITHIN it
// (cycle-1502). Vouch ONLY when the ancestor ledger exists with entries: the
// unblocked missing/empty branch verified NOTHING, and a deleted ancestor
// ledger must not convert the closure gate's backstop into a demotion. Reads
// the Doc the grade already loaded — no second read.
func vouchedCycles(cont continuation.Continuation, ancestor Doc) []int {
	if len(ancestor.Entries) == 0 {
		return nil
	}
	cycles := []int{cont.Cycle}
	if ancestor.OriginCycle > 0 && ancestor.OriginCycle != cont.Cycle {
		cycles = append(cycles, ancestor.OriginCycle)
	}
	return cycles
}
