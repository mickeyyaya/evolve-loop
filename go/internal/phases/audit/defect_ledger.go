package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// defect_ledger.go — the anti-laundering ledger
// (batch-integrity-review-2026-08-04.md F1(i)).
//
// A named CRITICAL defect survived the 1255 → 1268-salvage → 1270 → 1272 chain
// by being individually honest at every step but collectively erased: each
// continuation narrowed, renamed, or declared-already-fixed the defect, and no
// code anywhere required a continuation to reconcile against the ORIGINAL
// rejecting audit's machine-readable defects[].
//
// Two mechanisms, both hanging off hooks.Classify (the audit verdict seam):
//
//  1. EMIT — a rejecting audit persists <workspace>/defect-ledger.json, one
//     addressable OPEN entry per structured defect, text verbatim.
//  2. RECONCILE — a continuation cycle loads its ancestor's ledger and may NOT
//     emit PASS while any inherited OPEN entry is unaccounted for. The
//     disposition is written back into THIS cycle's ledger, so it is visible in
//     the audit's own artifact rather than inferable from a diff a human must
//     run. Entries transition; they are never deleted. A ledger that shrinks is
//     a ledger that launders.
//
// Degrade posture, deliberately asymmetric: a cycle that is not a continuation
// (no manifest) or whose ancestor left no ledger is a clean no-op — the
// overwhelming majority of cycles, and nothing to reconcile against. But a
// MISSING disposition artifact on a real continuation is the defect itself, not
// an environment gap, so it blocks (unlike probe_quarantine's missing-worktree
// case, which correctly degrades open).

const (
	defectLedgerFile      = "defect-ledger.json"
	defectDispositionFile = "defect-dispositions.json"

	defectStatusOpen     = "OPEN"
	defectStatusFixed    = "FIXED"
	defectStatusDeferred = "DEFERRED"

	// defectLedgerMaxEntries and defectTextMaxRunes bound the ledger against an
	// agent-authored verdict sentinel carrying thousands of defects or a
	// megabyte-long defect line (cycle-1282 DEF-6). The ledger is re-read and
	// re-written on every Classify, so unbounded growth is quadratic work on the
	// audit hot path as well as an unreadable artifact. Overflow is RECORDED as
	// a synthetic entry, never silently dropped — a cap that erases defects
	// would be the laundering primitive wearing a resource-limit costume.
	defectLedgerMaxEntries = 64
	defectTextMaxRunes     = 2000
)

// defectEntry is one tracked defect. Evidence is mandatory on FIXED and Reason
// on DEFERRED — an unevidenced closure claim is the laundering primitive.
type defectEntry struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// defectLedgerDoc is the on-disk <workspace>/defect-ledger.json wire shape.
// OriginCycle names the cycle that RAISED the defects, so a continuation can
// trace lineage back past its immediate ancestor.
type defectLedgerDoc struct {
	OriginCycle int           `json:"origin_cycle"`
	Entries     []defectEntry `json:"entries"`
}

// defectDispositionDoc is the on-disk <workspace>/defect-dispositions.json the
// continuation's builder/auditor writes: the claim, per inherited defect id.
type defectDispositionDoc struct {
	Dispositions []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Evidence string `json:"evidence"`
		Reason   string `json:"reason"`
	} `json:"dispositions"`
}

// readDefectLedger loads dir's ledger. Missing file → (zero, false, nil): a
// cycle with no ledger has nothing to reconcile. Present-but-unparseable is an
// error — schema drift on the anti-laundering record must be loud.
func readDefectLedger(dir string) (defectLedgerDoc, bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, defectLedgerFile))
	if err != nil {
		if os.IsNotExist(err) {
			return defectLedgerDoc{}, false, nil
		}
		return defectLedgerDoc{}, false, fmt.Errorf("read %s: %w", defectLedgerFile, err)
	}
	var doc defectLedgerDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return defectLedgerDoc{}, false, fmt.Errorf("parse %s: %w", defectLedgerFile, err)
	}
	return doc, true, nil
}

// writeDefectLedger persists doc atomically into dir.
func writeDefectLedger(dir string, doc defectLedgerDoc) error {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", defectLedgerFile, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s dir: %w", defectLedgerFile, err)
	}
	return atomicwrite.Bytes(filepath.Join(dir, defectLedgerFile), body)
}

// emitDefectLedger records this cycle's rejection as addressable OPEN entries.
// Sourced from the evolve-verdict sentinel's structured failure block — the
// same input extractAuditVerdict already parses, never a test-only side
// channel. A rejection with no structured defects mints nothing: an empty
// ledger on every cycle would make every later cycle look like a continuation
// and is the cheapest way to make the reconcile gate vacuous.
//
// New defects are APPENDED to any ledger already in the workspace (a
// continuation that inherits entries and then raises its own must keep both) —
// merge, never replace, because replacement is deletion by another name.
func emitDefectLedger(artifact string, req core.PhaseRequest) error {
	if req.Workspace == "" {
		return nil
	}
	s, ok := phasecontract.ParseVerdictSentinelFull(artifact)
	if !ok || s.Failure == nil || (len(s.Failure.Defects) == 0 && len(s.Failure.Prescription) == 0) {
		return nil
	}
	doc, existed, err := readDefectLedger(req.Workspace)
	if err != nil {
		return err
	}
	if !existed {
		doc.OriginCycle = req.Cycle
	}
	known := make(map[string]bool, len(doc.Entries))
	for _, e := range doc.Entries {
		known[e.Text] = true
	}
	added := false
	overflow := 0
	appendLedgerEntry := func(text string) {
		text = truncateRunes(text, defectTextMaxRunes)
		if known[text] {
			return // same defect/prescription re-reported on a retry — one row, not two
		}
		if len(doc.Entries) >= defectLedgerMaxEntries {
			overflow++
			return
		}
		known[text] = true
		doc.Entries = append(doc.Entries, defectEntry{
			ID:     defectID(text),
			Text:   text,
			Status: defectStatusOpen,
		})
		added = true
	}
	for _, text := range s.Failure.Defects {
		appendLedgerEntry(text)
	}
	for _, text := range s.Failure.Prescription {
		// Tagged distinguishably from Defects (F3, scout report Hypothesis
		// 2): a prescription describes a named fix for a foreseen risk, not
		// something itself wrong — an operator reading defect-ledger.json
		// must be able to tell the two apart without a second ledger or a
		// schema-breaking Kind field.
		appendLedgerEntry("PRESCRIPTION: " + text)
	}
	if overflow > 0 {
		// One OPEN row standing for the truncated tail. It has no per-defect
		// text, so it can never be dispositioned by a targeted claim — a
		// continuation inheriting it must widen the cap or fix the emitter,
		// which is the correct forcing function for an overflowing rejection.
		text := fmt.Sprintf("%d further defect(s) from cycle-%d were not recorded: the ledger cap of %d entries was reached", overflow, req.Cycle, defectLedgerMaxEntries)
		if !known[text] {
			doc.Entries = append(doc.Entries, defectEntry{ID: defectID(text), Text: text, Status: defectStatusOpen})
			added = true
		}
	}
	if !added {
		return nil
	}
	return writeDefectLedger(req.Workspace, doc)
}

// defectID derives an entry id from the defect TEXT alone. A positional id
// ("d"+index) re-binds the same id string to different text as soon as a list
// is reordered or an entry is added upstream, so a disposition keyed on that id
// closes something other than what it claims — laundering by renumbering. A
// content hash is stable across cycles, chains, and re-emissions: the same
// defect always answers to the same id, and two different defects never share
// one.
//
// SIXTEEN bytes, not four (cycle-1282 DEF-3). The preimage is fully chosen by
// the agent authoring the verdict sentinel, so a 32-bit id is a ~2^32 brute
// force away from minting a benign defect that collides with an inherited
// CRITICAL and shadows it in the merge index. 128 bits puts a second preimage
// out of reach; readability is not worth an id an adversary can aim. The merge
// additionally cross-checks TEXT per id, so a collision is loud rather than
// silent even at this width.
func defectID(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "d" + hex.EncodeToString(sum[:16])
}

// truncateRunes bounds s to at most max runes, marking any cut so a reader can
// tell a clipped defect line from a short one.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…[truncated]"
}

// evidenceResolves reports whether a closure claim's evidence names a file that
// actually EXISTS, plus the operator-facing reason when it does not. Validating
// evidence for non-emptiness alone accepts `evidence:"x"` and closes a CRITICAL
// on a string nobody can follow — the unverifiable closure claim the batch
// integrity review indicts.
//
// Deliberately permissive about SHAPE, strict about WHAT IT NAMES: auditors
// cite "path:line" and "path:line:col" as often as a bare path, and rejecting a
// legitimate citation shape would block every future continuation.
//
// Existence alone was the cycle-1282 DEF-2 hole: `os.Stat` under either root,
// plus a raw-absolute-path branch, meant `/etc/hosts` and the attacker's own
// `defect-dispositions.json` each closed a CRITICAL. Four rules now hold:
//
//  1. RELATIVE only. An absolute path names something outside the repo's
//     accounting; `/etc/hosts` exists on every host and proves nothing.
//  2. NO ESCAPE. After Clean, a leading ".." leaves the root — the workspace
//     sits three levels down, so traversal is reachable, not theoretical.
//  3. PROJECT ROOT or this lane's WORKTREE, never the workspace. A citation is
//     resolved under the project root first and, failing that, under
//     req.Worktree — an unmerged lane's fix is only ever in its own worktree
//     (cycle-1340; the 1320→1330 deadlock). The workspace is still barred: it is
//     this cycle's own agent-authored ephemera; citing it is the graded party
//     vouching for itself. Real workspace artifacts remain citable by their
//     path FROM the root (".evolve/runs/cycle-N/audit-report.md"), which is
//     also what makes the citation followable by a reader who has only the repo.
//  4. NOT THE GATE'S OWN RECORD. defect-dispositions.json / defect-ledger.json /
//     continuation-manifest.json are the mechanism's own bookkeeping; a claim
//     that cites them cites itself.
//
// Symlinks are rejected with Lstat rather than followed: a symlink planted in
// the tree resolves rule 2 away.
//
// minimal: ceiling is "a real, in-repo, non-self file exists". It does NOT
// prove the file is in this cycle's diff or is related to the defect text.
// Upgrade path: resolve against the changed set (`git diff --name-only
// <manifest.base_sha>` in the worktree) once the audit hook carries the diff.
func evidenceResolves(citation string, req core.PhaseRequest) (bool, string) {
	path := strings.TrimSpace(citation)
	if path == "" {
		return false, "no evidence"
	}
	// Strip at most a ":line" and a ":col" suffix; anything else is part of the
	// path (a Windows drive letter is not reachable here — these are repo paths).
	// A ":line-line" RANGE counts as one locator: it is the house citation style
	// in build and audit reports, and leaving it glued to the path made every
	// range citation unresolvable under EVERY root (cycle-1340, defect
	// ddda7857a — a real file, present in both roots, rejected anyway).
	for i := 0; i < 2; i++ {
		idx := strings.LastIndex(path, ":")
		if idx <= 0 || !isLineLocator(path[idx+1:]) {
			break
		}
		path = path[:idx]
	}

	if filepath.IsAbs(path) {
		return false, fmt.Sprintf("evidence %q is an absolute path — closure evidence must name a repo-relative file so a reader can follow it", citation)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false, fmt.Sprintf("evidence %q escapes the project root", citation)
	}
	// Case-INSENSITIVE (cycle-1285 F3). The rejection below and the os.Lstat
	// two lines down must agree on what "the same file" means, and on the
	// stated platform (darwin/APFS) Lstat resolves "Defect-Ledger.json" to
	// defect-ledger.json while an exact-string switch does not. That gap let
	// the gate's OWN record close every inherited defect. Comparing with
	// EqualFold is strictly conservative: on a case-sensitive volume it can
	// only reject a differently-cased name that was never going to be a
	// legitimate citation anyway.
	base := filepath.Base(clean)
	for _, own := range []string{defectLedgerFile, defectDispositionFile, "continuation-manifest.json"} {
		if strings.EqualFold(base, own) {
			return false, fmt.Sprintf("evidence %q cites the defect-ledger mechanism's own bookkeeping — a closure claim may not vouch for itself", citation)
		}
	}
	if req.ProjectRoot == "" {
		return false, fmt.Sprintf("evidence %q cannot be resolved: no project root on the phase request", citation)
	}
	// Two roots, project root FIRST. A continuation lane's fix lives in the
	// lane's own worktree and reaches the project root only when the lane
	// merges — which is exactly what this gate blocks when the citation
	// misses. Cycles 1320→1323→1325→1330 each cited a real, worktree-resident
	// file and each was rejected identically: the gate demanded evidence it
	// structurally prevented from existing. The worktree is a FALLBACK, not a
	// replacement, and it is reached only after rules 1-4 above have already
	// run — so a self-citation or an escape is refused under either root.
	roots := []string{req.ProjectRoot}
	if req.Worktree != "" && req.Worktree != req.ProjectRoot {
		roots = append(roots, req.Worktree)
	}
	var lastMode os.FileMode
	sawIrregular := false
	for _, root := range roots {
		info, err := os.Lstat(filepath.Join(root, clean))
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			lastMode, sawIrregular = info.Mode(), true
			continue
		}
		return true, ""
	}
	if sawIrregular {
		return false, fmt.Sprintf("evidence %q is not a regular file (mode %s)", citation, lastMode)
	}
	if len(roots) == 1 {
		return false, fmt.Sprintf("evidence %q resolves to no file under the project root", citation)
	}
	return false, fmt.Sprintf("evidence %q resolves to no file under the project root or this lane's worktree", citation)
}

// isLineLocator reports whether s is a source locator that may be shaved off a
// citation: a line number ("570") or a line range ("570-588"). Anything else —
// including a bare "-", "570-" or "notaline" — is part of the filename and is
// kept, so a claim can never be satisfied by a different file than it names.
func isLineLocator(s string) bool {
	if isAllDigits(s) {
		return true
	}
	lo, hi, ok := strings.Cut(s, "-")
	return ok && isAllDigits(lo) && isAllDigits(hi)
}

// isAllDigits reports whether s is a non-empty run of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// reconcileContinuationDefects is the disposition diff. It returns the
// diagnostics to surface and whether the cycle must be blocked from PASS.
//
// Blocking cases are exactly the ones where an inherited defect could otherwise
// vanish: an unaccounted OPEN entry, an unevidenced closure claim, an
// unreadable ancestor ledger, or a write-back failure (a disposition that did
// not reach disk is not visible, which is the whole point of the mechanism).
func reconcileContinuationDefects(req core.PhaseRequest) ([]core.Diagnostic, bool) {
	if req.Workspace == "" || req.ProjectRoot == "" {
		return nil, false
	}
	cont, isContinuation, err := continuation.ReadManifest(req.Workspace)
	if err != nil {
		// cycle-1285 F2. This used to degrade OPEN with a warning: one byte of
		// garbage written into a 0644 file in the graded agent's own workspace
		// retired every inherited defect and the cycle PASSed. readDispositions
		// already BLOCKS on an unparseable defect-dispositions.json because
		// "degrading open there would hand the gate its cheapest bypass" — and
		// this file decides whether the gate runs AT ALL, so the same argument
		// applies with more force, not less. A corrupt manifest is now a
		// blocking error; the cost is that a genuinely corrupt workspace must be
		// repaired before the cycle can PASS, which is the correct direction to
		// fail in for a mechanism whose entire job is to make defects visible.
		return []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: continuation manifest is unreadable (%s) — a continuation cannot be graded against a lineage it cannot read, and degrading open here is the gate's cheapest bypass", err.Error())}}, true
	}
	// cycle-1285 F2, the deletion half. Arming may not depend SOLELY on a file
	// inside the workspace the graded agent writes: `rm continuation-manifest.json`
	// closed every inherited OPEN defect with verdict PASS and zero diagnostics.
	// The lineage is also witnessed out of band, in the root-owned
	// continuation-registry.json (ADR-0076 slice C G2) keyed by this lane's
	// scope, so the manifest is a workspace CACHE of a record that survives it.
	registryCont, hasRegistry := laneRegistryBinding(req)
	switch {
	case !isContinuation && !hasRegistry:
		return nil, false
	case !isContinuation && hasRegistry:
		graded, _ := reconcileAgainstAncestor(req, registryCont)
		return append([]core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: this workspace holds no continuation manifest, but the root-owned %s binds this lane's scope to cycle-%d — the manifest was deleted or never written. Inherited defects are reconciled from the registry binding; the missing manifest is itself the finding.",
				continuation.RegistryPath(req.ProjectRoot), registryCont.Cycle)}},
			graded...), true
	case hasRegistry && registryCont.Cycle != cont.Cycle:
		// Both records exist and disagree about the ancestor. The workspace copy
		// is the rewritable one, so it is the suspect; refusing to pick is the
		// only honest move.
		return []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: the workspace continuation manifest names cycle-%d but the root-owned registry binds this lane to cycle-%d — a rewritten manifest would re-point the gate at an ancestor with no open defects; resolve the disagreement before this cycle can PASS",
				cont.Cycle, registryCont.Cycle)}}, true
	}

	return reconcileAgainstAncestor(req, cont)
}

// laneRegistryBinding returns this lane's continuation binding from the
// ROOT-OWNED registry, and whether one exists.
//
// The lane's identity is its pinned scope (lane-scope.json, materialized before
// any phase runs), and the binding itself lives at the project root, outside
// every per-cycle workspace. That split is the point: an agent that deletes its
// workspace manifest does not thereby delete the record that this lane is a
// continuation. Scoping the lookup to THIS lane's todo ids — rather than
// arming on any entry in the registry — is what keeps the fallback from
// blocking ordinary cycles, whose root registry legitimately still carries
// other lanes' bindings.
//
// Fail-closed is not available here: an unreadable registry cannot manufacture
// a lineage, so a miss is a miss. The residual is a workspace where BOTH the
// manifest and the lane-scope pin are destroyed; that is recorded as the
// mechanism's known ceiling in docs/architecture/continuation-defect-ledger.md
// rather than papered over.
func laneRegistryBinding(req core.PhaseRequest) (continuation.Continuation, bool) {
	raw, err := os.ReadFile(filepath.Join(req.Workspace, core.LaneScopeFile))
	if err != nil {
		return continuation.Continuation{}, false
	}
	var scope core.LaneScope
	if err := json.Unmarshal(raw, &scope); err != nil {
		return continuation.Continuation{}, false
	}
	for _, id := range scope.TodoIDs {
		c, ok, rerr := continuation.ReadRegistryEntry(req.ProjectRoot, id)
		if rerr == nil && ok {
			return c, true
		}
	}
	return continuation.Continuation{}, false
}

// reconcileAgainstAncestor is the disposition diff proper: given an established
// lineage, it grades this cycle's dispositions against the ancestor's ledger.
// Split from reconcileContinuationDefects so that ARMING (is this a
// continuation, and which record establishes that) is one decision with one set
// of rules, separate from grading — the cycle-1285 F2 defect was entirely in
// the arming half while every defense lived in this half.
func reconcileAgainstAncestor(req core.PhaseRequest, cont continuation.Continuation) ([]core.Diagnostic, bool) {
	ancestorWS := filepath.Join(req.ProjectRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(cont.Cycle))
	ancestor, hasLedger, err := readDefectLedger(ancestorWS)
	if err != nil {
		return []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: ancestor cycle-%d ledger is unreadable (%s) — a continuation cannot be graded against a ledger it cannot read", cont.Cycle, err.Error())}}, true
	}
	if !hasLedger || len(ancestor.Entries) == 0 {
		// Nothing inherited. Legitimate when the ancestor predates the ledger —
		// but a DELETED ancestor ledger is indistinguishable from that, and one
		// `rm` outside the workspace would otherwise disarm the whole gate in
		// silence. Recording it by ancestor cycle number makes the disarm
		// visible in the audit's own diagnostics without blocking the many real
		// continuations whose ancestors ran before the mechanism existed.
		return []core.Diagnostic{{Severity: "warning",
			Message: fmt.Sprintf("defect ledger: this cycle continues cycle-%d, which left no reconcilable %s in %s — NO inherited defect is being enforced here. Expected for an ancestor that predates the ledger; a deleted ledger looks identical, so it is recorded rather than assumed benign.",
				cont.Cycle, defectLedgerFile, ancestorWS)}}, false
	}

	// D1: reconcile MERGES onto the ledger already in this workspace. Rebuilding
	// from ancestor.Entries alone and truncate-writing erases the entries emit
	// appended on a previous Classify in this same cycle — an ordinary audit
	// retry silently deletes the record of what THIS cycle got wrong. Entries
	// transition; they are never deleted.
	current, _, cerr := readDefectLedger(req.Workspace)
	if cerr != nil {
		return []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: this cycle's own %s is unreadable (%s) — reconciling would overwrite a record that cannot be read", defectLedgerFile, cerr.Error())}}, true
	}

	claims, diags, blocked := readDispositions(req.Workspace, cont.Cycle)
	if blocked {
		return diags, true
	}
	diags = append(diags, dispositionPreflight(req, cont.Cycle, ancestor.Entries, claims)...)

	// D1 / cycle-1282 DEF-1: the inherited rows are rebuilt from the ANCESTOR on
	// every pass and their status is derived ONLY from defect-dispositions.json,
	// never from the row already sitting in this workspace. `current` is a file
	// the graded phase agent is permitted to write, so reading disposition state
	// out of it let a pre-planted `{"id":"d1","status":"FIXED"}` satisfy the gate
	// with no disposition artifact at all — and, because the merge keyed on ID
	// alone, replace the inherited defect's TEXT with the planted row's. What
	// `current` still contributes is THIS cycle's own emitted defects, which are
	// not inherited and are not graded here.
	//
	// Idempotency across retries in one cycle does not need the trusted-row
	// shortcut: defect-dispositions.json persists, so every Classify re-derives
	// the same dispositions from the same artifact and re-validates them.
	merged := append([]defectEntry(nil), current.Entries...)
	pos := make(map[string]int, len(merged)+len(ancestor.Entries))
	for i, e := range merged {
		if _, dup := pos[e.ID]; dup {
			continue // FIRST row wins; a later duplicate must not become the index target
		}
		pos[e.ID] = i
	}

	var unaccounted []string
	for _, a := range ancestor.Entries {
		i, carried := pos[a.ID]
		if !carried {
			i = len(merged)
			pos[a.ID] = i
			merged = append(merged, a)
		} else if merged[i].Text != a.Text {
			// Same id, different text: either a defectID collision or a planted
			// row aimed at an inherited id. Both must be loud, and in both cases
			// the ANCESTOR's text is the record. Blocking is correct — a shadowed
			// id means the operator cannot trust any disposition keyed on it.
			unaccounted = append(unaccounted, fmt.Sprintf("%s (id shadowed: this cycle's ledger holds different text %q for the same id)", a.ID, truncateRunes(merged[i].Text, 120)))
			merged[i] = defectEntry{ID: a.ID, Text: a.Text, Status: defectStatusOpen}
			continue
		}
		if a.Status != defectStatusOpen {
			merged[i] = a // dispositioned upstream — carried verbatim, evidence and reason included
			continue
		}
		// Fresh from the ancestor, never from the workspace row.
		e := defectEntry{ID: a.ID, Text: a.Text, Status: defectStatusOpen}
		claim, has := claims[a.ID]
		switch {
		case !has:
			unaccounted = append(unaccounted, a.ID+" (no disposition)")
			e.Status = defectStatusOpen
		case claim.Status == defectStatusFixed:
			if ok, why := evidenceResolves(claim.Evidence, req); !ok {
				unaccounted = append(unaccounted, fmt.Sprintf("%s (FIXED but %s)", a.ID, why))
				// The written-back artifact must not assert a closure the gate
				// rejected — an unverifiable FIXED row IS the laundering.
				e.Status, e.Evidence, e.Reason = defectStatusOpen, "", ""
			} else {
				e.Status, e.Evidence, e.Reason = claim.Status, claim.Evidence, claim.Reason
			}
		case claim.Status == defectStatusDeferred:
			if strings.TrimSpace(claim.Reason) == "" {
				unaccounted = append(unaccounted, a.ID+" (DEFERRED without reason)")
				e.Status = defectStatusOpen
			} else {
				e.Status, e.Evidence, e.Reason = claim.Status, claim.Evidence, claim.Reason
			}
		default:
			unaccounted = append(unaccounted, fmt.Sprintf("%s (status %q is not FIXED or DEFERRED)", a.ID, claim.Status))
			e.Status = defectStatusOpen
		}
		merged[i] = e // an unaccounted entry stays OPEN — it is never dropped
	}

	originCycle := ancestor.OriginCycle
	if originCycle == 0 {
		originCycle = current.OriginCycle
	}

	// Write back BEFORE grading: the operator must be able to read what this
	// cycle disposed of even on the run where the gate blocks.
	if werr := writeDefectLedger(req.Workspace, defectLedgerDoc{OriginCycle: originCycle, Entries: merged}); werr != nil {
		diags = append(diags, core.Diagnostic{Severity: "error",
			Message: fmt.Sprintf("defect ledger: could not write back the reconciled ledger (%s) — an invisible disposition is not a disposition", werr.Error())})
		return diags, true
	}
	if len(unaccounted) > 0 {
		diags = append(diags, core.Diagnostic{Severity: "error",
			Message: fmt.Sprintf("defect ledger: %d defect(s) inherited from cycle-%d are unaccounted for [%s] — a continuation may not PASS while an ancestor defect is neither FIXED (with evidence) nor DEFERRED (with a reason). Disposition each id in %s.",
				len(unaccounted), cont.Cycle, strings.Join(unaccounted, ", "), defectDispositionFile)})
		return diags, true
	}
	return diags, false
}

// The two NAMED markers the disposition pre-flight emits. They are deliberately
// distinct from the per-id "(no disposition)" switch text: an operator reading a
// blocked continuation must be able to see that the ARTIFACT as a whole is
// absent or short, not infer it from N unrelated-looking per-id gripes. MISSING
// and INCOMPLETE stay separate because the operator action differs — author the
// file from scratch vs finish the one that exists.
const (
	dispositionPreflightMissingMarker    = "disposition-preflight: MISSING"
	dispositionPreflightIncompleteMarker = "disposition-preflight: INCOMPLETE"
)

// dispositionPreflight grades the disposition ARTIFACT's completeness against
// the ancestor's OPEN set, before the per-id reconcile runs (cycle-1342 F4).
// The per-id switch already blocks correctly; what it never did was fail loudly
// BY NAME on the file itself, so a future auditor that simply forgets to write
// it reads N per-id complaints and no statement of the actual gap.
//
// It is silent — necessarily, as the anti-no-op half — whenever the ancestor
// carries no OPEN entries or every one of them is covered. A pre-flight that
// fires on every continuation proves nothing.
func dispositionPreflight(req core.PhaseRequest, ancestorCycle int, ancestor []defectEntry, claims map[string]defectEntry) []core.Diagnostic {
	var open, uncovered []string
	for _, a := range ancestor {
		if a.Status != defectStatusOpen {
			continue // already dispositioned upstream — nothing is owed for it here
		}
		open = append(open, a.ID)
		if _, has := claims[a.ID]; !has {
			uncovered = append(uncovered, a.ID)
		}
	}
	if len(open) == 0 || len(uncovered) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(req.Workspace, defectDispositionFile)); err != nil {
		return []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: %s — this workspace holds no %s at all, so 0 of %d defect(s) inherited from cycle-%d are dispositioned. This file is re-authored IN FULL every cycle; an ancestor's copy is never inherited. Write one entry per inherited id, status FIXED (with resolvable evidence) or DEFERRED (with a reason).",
				dispositionPreflightMissingMarker, defectDispositionFile, len(open), ancestorCycle)}}
	}
	return []core.Diagnostic{{Severity: "error",
		Message: fmt.Sprintf("defect ledger: %s — %s covers %d of %d defect(s) inherited from cycle-%d; uncovered: [%s]. Every inherited id needs its own entry in THIS cycle's file.",
			dispositionPreflightIncompleteMarker, defectDispositionFile, len(open)-len(uncovered), len(open), ancestorCycle, strings.Join(uncovered, ", "))}}
}

// readDispositions loads the continuation's disposition claims keyed by defect
// id. A MISSING file is not an error and not a pass: it yields an empty map, so
// every inherited OPEN entry falls through to "unaccounted" and is named by id.
// An unparseable file blocks immediately — degrading open there would hand the
// gate its cheapest bypass (write garbage, ship).
func readDispositions(workspace string, ancestorCycle int) (map[string]defectEntry, []core.Diagnostic, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, defectDispositionFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]defectEntry{}, []core.Diagnostic{{Severity: "warning",
				Message: fmt.Sprintf("defect ledger: no %s in the workspace — every defect inherited from cycle-%d is unaccounted for", defectDispositionFile, ancestorCycle)}}, false
		}
		return nil, []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: read %s: %s", defectDispositionFile, err.Error())}}, true
	}
	var doc defectDispositionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, []core.Diagnostic{{Severity: "error",
			Message: fmt.Sprintf("defect ledger: %s is unparseable (%s) — a continuation cannot be graded against claims that cannot be read", defectDispositionFile, err.Error())}}, true
	}
	claims := make(map[string]defectEntry, len(doc.Dispositions))
	for _, d := range doc.Dispositions {
		claims[d.ID] = defectEntry{ID: d.ID, Status: d.Status, Evidence: d.Evidence, Reason: d.Reason}
	}
	return claims, nil, false
}
