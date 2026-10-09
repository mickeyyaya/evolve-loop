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

const (
	defectLedgerFile      = defectledger.LedgerFile
	defectDispositionFile = defectledger.DispositionsFile
)

const (
	defectStatusOpen     = defectledger.StatusOpen
	defectStatusFixed    = defectledger.StatusFixed
	defectStatusDeferred = defectledger.StatusDeferred
)

const (
	defectLedgerMaxEntries = defectledger.MaxEntries
	defectTextMaxRunes     = defectledger.TextMaxRunes
)

const (
	dispositionPreflightMissingMarker    = defectledger.PreflightMissingMarker
	dispositionPreflightIncompleteMarker = defectledger.PreflightIncompleteMarker
	dispositionSchemaExample             = defectledger.DispositionsSchemaExample
)

type (
	defectEntry     = defectledger.Entry
	defectLedgerDoc = defectledger.Doc
)

func (h hooks) defectLedger() *defectledger.Ledger {
	if h.ledger != nil {
		return h.ledger
	}
	return nullDefectLedger()
}

func wiredDefectLedger(signals func() *signalcenter.Center) *defectledger.Ledger {
	return defectledger.New(core.LaneScopeIDs, resolveEvidence, defectledger.WithSignals(signals))
}

func nullDefectLedger() *defectledger.Ledger { return wiredDefectLedger(nil) }

func ledgerRequest(req core.PhaseRequest) defectledger.Request {
	return defectledger.Request{Cycle: req.Cycle, Workspace: req.Workspace, ProjectRoot: req.ProjectRoot, Worktree: req.Worktree}
}

func rejectionOf(artifact string) (defectledger.Rejection, bool) {
	s, ok := phasecontract.ParseVerdictSentinelFull(artifact)
	if !ok || s.Failure == nil {
		return defectledger.Rejection{}, false
	}
	return defectledger.Rejection{Defects: s.Failure.Defects, Prescriptions: s.Failure.Prescription}, true
}

func resolveEvidence(evidence string, r defectledger.Request) (bool, string) {
	return evidenceResolves(evidence, core.PhaseRequest{ProjectRoot: r.ProjectRoot, Worktree: r.Worktree})
}

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

func truncateRunes(s string, max int) string { return defectledger.Truncate(s, max) }

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

func splitEvidence(evidence string) []string {
	var out []string
	for _, part := range strings.Split(evidence, ";") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func oneEvidenceResolves(citation string, req core.PhaseRequest) (bool, string) {
	trimmed := strings.TrimSpace(citation)
	if trimmed == "" {
		return false, "no evidence"
	}
	path := stripCitationLocator(trimmed)
	if filepath.IsAbs(path) {
		return false, fmt.Sprintf("evidence %q is an absolute path — closure evidence must name a repo-relative file so a reader can follow it", citation)
	}
	clean := filepath.Clean(path)
	if escapesRoot(clean) {
		return false, fmt.Sprintf("evidence %q escapes the project root", citation)
	}
	if isOwnBookkeeping(clean) {
		return false, fmt.Sprintf("evidence %q cites the defect-ledger mechanism's own bookkeeping — a closure claim may not vouch for itself", citation)
	}
	if req.ProjectRoot == "" {
		return false, fmt.Sprintf("evidence %q cannot be resolved: no project root on the phase request", citation)
	}
	roots := citationRoots(req)
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

func stripCitationLocator(path string) string {
	if strings.HasSuffix(path, ")") {
		if i := strings.LastIndex(path, " ("); i > 0 {
			path = strings.TrimSpace(path[:i])
		}
	}
	for range 2 {
		idx := strings.LastIndex(path, ":")
		if idx <= 0 || !isLineLocator(path[idx+1:]) {
			break
		}
		path = path[:idx]
	}
	return path
}

func escapesRoot(clean string) bool {
	return clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func isOwnBookkeeping(clean string) bool {
	base := filepath.Base(clean)
	for _, own := range []string{defectLedgerFile, defectDispositionFile, continuation.ManifestName} {
		if strings.EqualFold(base, own) {
			return true
		}
	}
	return false
}

func citationRoots(req core.PhaseRequest) []string {
	roots := []string{req.ProjectRoot}
	if req.Worktree != "" && req.Worktree != req.ProjectRoot {
		roots = append(roots, req.Worktree)
	}
	return roots
}

func isLineLocator(s string) bool {
	if isAllDigits(s) {
		return true
	}
	lo, hi, ok := strings.Cut(s, "-")
	return ok && isAllDigits(lo) && isAllDigits(hi)
}

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
