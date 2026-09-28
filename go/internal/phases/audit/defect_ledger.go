package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The ledger's vocabulary, projected (single source: the leaf).
const (
	defectLedgerFile      = defectledger.LedgerFile
	defectDispositionFile = defectledger.DispositionsFile
)

// The status vocabulary, projected.
const (
	defectStatusOpen     = defectledger.StatusOpen
	defectStatusFixed    = defectledger.StatusFixed
	defectStatusDeferred = defectledger.StatusDeferred
)

// The ledger's bounds, projected (audit_report_length_test.go reads them).
const (
	defectLedgerMaxEntries = defectledger.MaxEntries
	defectTextMaxRunes     = defectledger.TextMaxRunes
)

// The pre-flight markers and the schema example, projected (the bookkeeping
// regrade, the seed single-source pin and the doc-example tests read them).
const (
	dispositionPreflightMissingMarker    = defectledger.PreflightMissingMarker
	dispositionPreflightIncompleteMarker = defectledger.PreflightIncompleteMarker
	dispositionSchemaExample             = defectledger.DispositionsSchemaExample
)

// defectEntry and defectLedgerDoc keep the tests' spellings of the wire
// shape; an alias carries no second tag set.
type (
	defectEntry     = defectledger.Entry
	defectLedgerDoc = defectledger.Doc
)

// defectLedger is the nil-safe accessor: the wired ledger New built, or the
// Null-Object ledger for a hooks{} literal, since Classify and ComposePrompt's
// value receivers cannot cache one.
func (h hooks) defectLedger() *defectledger.Ledger {
	if h.ledger != nil {
		return h.ledger
	}
	return nullDefectLedger()
}

func wiredDefectLedger(signals func() *signalcenter.Center) *defectledger.Ledger {
	return defectledger.New(core.LaneScopeIDs, resolveEvidence, defectledger.WithSignals(signals))
}

// nullDefectLedger is the ledger with no Center: the registry root (`evolve
// phase audit`, which carries no Center for any phase) and the test-only
// facades run on it.
func nullDefectLedger() *defectledger.Ledger { return wiredDefectLedger(nil) }

// ledgerRequest is the ONE projection of the phase request onto the ledger's:
// exactly the four fields the gate reads.
func ledgerRequest(req core.PhaseRequest) defectledger.Request {
	return defectledger.Request{Cycle: req.Cycle, Workspace: req.Workspace, ProjectRoot: req.ProjectRoot, Worktree: req.Worktree}
}

// rejectionOf parses the verdict sentinel's structured failure block — the
// same input extractAuditVerdict already parses, never a test-only side
// channel; false when the artifact carries no failure block.
func rejectionOf(artifact string) (defectledger.Rejection, bool) {
	s, ok := phasecontract.ParseVerdictSentinelFull(artifact)
	if !ok || s.Failure == nil {
		return defectledger.Rejection{}, false
	}
	return defectledger.Rejection{Defects: s.Failure.Defects, Prescriptions: s.Failure.Prescription}, true
}

// resolveEvidence adapts the audit resolver to the ledger's Strategy: the
// four rules read the project root and this lane's worktree only.
func resolveEvidence(evidence string, r defectledger.Request) (bool, string) {
	return evidenceResolves(evidence, core.PhaseRequest{ProjectRoot: r.ProjectRoot, Worktree: r.Worktree})
}

// The three production spellings (disposition.go, audit.go's ComposePrompt):
// the wired ledger, handed in by the caller.

func emitDefectLedgerVia(l *defectledger.Ledger, artifact string, req core.PhaseRequest) []core.Diagnostic {
	r, ok := rejectionOf(artifact)
	if !ok {
		return nil
	}
	return l.Emit(ledgerRequest(req), r).Diagnostics
}

func reconcileContinuationDefectsVia(l *defectledger.Ledger, req core.PhaseRequest) ([]core.Diagnostic, bool, []int) {
	v := l.Reconcile(ledgerRequest(req))
	return v.Diagnostics, v.Blocked, v.LineageCycles
}

func inheritedDefectsPromptBlockVia(l *defectledger.Ledger, req core.PhaseRequest) string {
	return l.PromptBlock(ledgerRequest(req))
}

// The Null-Object facades: the by-name tests and the ACS predicates keep
// these spellings; no production caller uses them.

func emitDefectLedger(artifact string, req core.PhaseRequest) []core.Diagnostic {
	return emitDefectLedgerVia(nullDefectLedger(), artifact, req)
}

func reconcileContinuationDefects(req core.PhaseRequest) ([]core.Diagnostic, bool, []int) {
	return reconcileContinuationDefectsVia(nullDefectLedger(), req)
}

func readDispositions(workspace string, ancestorCycle int) (map[string]defectEntry, []core.Diagnostic, bool) {
	return nullDefectLedger().ReadDispositions(defectledger.Request{Workspace: workspace}, ancestorCycle)
}

func dispositionPreflight(req core.PhaseRequest, ancestorCycle int, ancestor []defectEntry, claims map[string]defectEntry) []core.Diagnostic {
	return nullDefectLedger().Preflight(ledgerRequest(req), ancestorCycle, ancestor, claims)
}

func defectID(text string) string { return defectledger.ID(text) }

// truncateRunes is the audit rune cap — the ledger's, projected for
// closure_claim.go's quoteClaim and the evidence-shape error.
func truncateRunes(s string, max int) string { return defectledger.Truncate(s, max) }

// evidenceResolves reports whether a closure claim's evidence names a file
// that actually EXISTS, plus the operator-facing reason when it does not.
//
// Deliberately permissive about SHAPE, strict about WHAT IT NAMES: auditors
// cite "path:line" and "path:line:col" as often as a bare path, and rejecting
// a legitimate citation shape would block every future continuation. Four
// rules hold:
//
//  1. RELATIVE only. An absolute path names something outside the repo's
//     accounting; `/etc/hosts` exists on every host and proves nothing.
//  2. NO ESCAPE. After Clean, a leading ".." leaves the root.
//  3. PROJECT ROOT or this lane's WORKTREE, never the workspace. A citation is
//     resolved under the project root first and, failing that, under
//     req.Worktree — an unmerged lane's fix is only ever in its own worktree.
//     The workspace is still barred: it is this cycle's own agent-authored
//     ephemera; citing it is the graded party vouching for itself. Real
//     workspace artifacts remain citable by their path FROM the root
//     (".evolve/runs/cycle-N/audit-report.md"), which is also what makes the
//     citation followable by a reader who has only the repo.
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
// A multi-citation value (the array shape above, joined) resolves only when
// EVERY citation resolves: "one of these files exists" would let a real cite
// carry an invented one past the gate.
func evidenceResolves(evidence string, req core.PhaseRequest) (bool, string) {
	frags := splitEvidence(evidence)
	if len(frags) == 0 {
		return false, "no evidence"
	}
	cites := 0
	for _, f := range frags {
		// A ';'-joined fragment that is not cite-shaped is a prose ANNOTATION
		// ("…; verified live: `go test` -> PASS"), ignored rather than
		// graded; every cite-SHAPED fragment must still resolve, and at
		// least one is mandatory — prose alone stays inadmissible.
		if !citeShaped(f) {
			continue
		}
		cites++
		if ok, why := oneEvidenceResolves(f, req); !ok {
			return false, why
		}
	}
	if cites == 0 {
		return false, "no citation among annotations — prose alone is not evidence"
	}
	return true, ""
}

// citeShaped reports whether a fragment is graded as a citation (must resolve)
// rather than a prose annotation (ignored). Conservative by design: anything
// whitespace-free that looks path-like (contains '/' or '.') is a citation, so
// a typoed path can never demote itself into unfalsifiable prose. The optional
// trailing parenthetical annotation is stripped first — the same allowance
// oneEvidenceResolves grants a lone cite.
func citeShaped(frag string) bool {
	s := strings.TrimSpace(frag)
	if i := strings.LastIndex(s, " ("); i > 0 && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[:i])
	}
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	return strings.ContainsAny(s, "/.")
}

// splitEvidence returns the individual fragments of a (possibly joined)
// evidence value — citations and prose annotations alike; citeShaped decides
// which get graded. Blanks are dropped, so "", " " and "; " are all "no
// evidence" rather than a citation named "".
func splitEvidence(evidence string) []string {
	var out []string
	for _, part := range strings.Split(evidence, ";") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// oneEvidenceResolves applies the four rules to a SINGLE citation.
func oneEvidenceResolves(citation string, req core.PhaseRequest) (bool, string) {
	path := strings.TrimSpace(citation)
	if path == "" {
		return false, "no evidence"
	}
	// Drop ONE trailing parenthetical annotation ("path:114-129 (helperName
	// now cycle-scoped)") before locator stripping. The annotation is
	// dropped, never resolved; every rejection below still applies to the
	// stripped path — an annotation-only cite (" (…)" with nothing before
	// it, LastIndex 0) and a bare "(…)" (no " (" separator) fall through
	// unchanged and fail the path checks as before.
	if strings.HasSuffix(path, ")") {
		if i := strings.LastIndex(path, " ("); i > 0 {
			path = strings.TrimSpace(path[:i])
		}
	}
	// Strip at most a ":line" and a ":col" suffix; anything else is part of the
	// path (a Windows drive letter is not reachable here — these are repo paths).
	// A ":line-line" RANGE counts as one locator: it is the house citation style
	// in build and audit reports.
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
	// Case-INSENSITIVE. This rejection and the os.Lstat two lines down must
	// agree on what "the same file" means: on darwin/APFS, Lstat resolves
	// "Defect-Ledger.json" to defect-ledger.json while an exact-string switch
	// would not. Comparing with EqualFold is strictly conservative: on a
	// case-sensitive volume it can only reject a differently-cased name that
	// was never going to be a legitimate citation anyway.
	base := filepath.Base(clean)
	for _, own := range []string{defectLedgerFile, defectDispositionFile, continuation.ManifestName} {
		if strings.EqualFold(base, own) {
			return false, fmt.Sprintf("evidence %q cites the defect-ledger mechanism's own bookkeeping — a closure claim may not vouch for itself", citation)
		}
	}
	if req.ProjectRoot == "" {
		return false, fmt.Sprintf("evidence %q cannot be resolved: no project root on the phase request", citation)
	}
	// Two roots, project root FIRST. A continuation lane's fix lives in the
	// lane's own worktree and reaches the project root only when the lane
	// merges. The worktree is a FALLBACK, not a replacement, and it is
	// reached only after rules 1-4 above have already run — so a
	// self-citation or an escape is refused under either root.
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
