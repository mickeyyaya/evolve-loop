package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// disposition_preflight_test.go — RED contract for cycle-1342 Task 3
// `disposition-completeness-preflight` (scout-report.md Finding 4).
//
// Today, an ancestor id with no matching claim in defect-dispositions.json
// surfaces ONLY as a per-id fallthrough inside reconcileAgainstAncestor's
// switch — `unaccounted = append(unaccounted, a.ID+" (no disposition)")` —
// mixed in among every other per-id branch (FIXED-but-unresolvable,
// DEFERRED-without-reason, unknown status). That blocks PASS correctly, but
// there is no STRUCTURAL signal that names the disposition FILE itself as
// absent or short of the inherited-id set — a future auditor reading
// diagnostics sees N unrelated-looking per-id gripes, never "the file you
// were supposed to write covers 0 of 2 ids". Finding 4 calls this a
// pre-flight gap: nothing fails loudly, BY NAME, on the artifact's
// completeness as a whole before grading proceeds id-by-id.
//
// Every assertion below reaches its subject through the REAL production
// seam, hooks{}.Classify — the audit phase's verdict path. A helper called
// directly would pass on dead code.
//
// Adversarial diversity: negative (file entirely missing), negative (file
// present but short), edge/anti-no-op (a complete file and a non-continuation
// cycle must trip NEITHER new message — a pre-flight that always fires
// proves nothing).

// dispositionPreflightMissing and dispositionPreflightIncomplete are the two
// distinct, NAMED diagnostic markers this contract pins. They must not
// collide with the existing "(no disposition)" per-id text, and must be
// distinguishable from one another (entirely-absent vs partially-covered
// are different operator actions: author the file vs finish it).
const (
	dispositionPreflightMissing    = "disposition-preflight: MISSING"
	dispositionPreflightIncomplete = "disposition-preflight: INCOMPLETE"
)

// TestClassify_DispositionPreflightMissingFileIsNamed — NEGATIVE. A
// continuation inherits two OPEN defects from its ancestor and the
// continuation's workspace has NO defect-dispositions.json at all. The
// verdict must not PASS (already true today via the per-id switch), but the
// diagnostics must ALSO carry a structural marker naming the file itself as
// missing — not only two independent "(no disposition)" per-id lines.
func TestClassify_DispositionPreflightMissingFileIsNamed(t *testing.T) {
	_, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})
	// Deliberately do NOT write defect-dispositions.json.

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: a continuation with two unaccounted inherited defects must not PASS")
	}
	text := diagsText(diags)
	if !strings.Contains(text, dispositionPreflightMissing) {
		t.Errorf("AC1 unmet — diagnostics must name the structural pre-flight finding %q when defect-dispositions.json is entirely absent on a continuation cycle, distinct from the per-id \"(no disposition)\" switch text. diagnostics:\n%s", dispositionPreflightMissing, text)
	}
	if !strings.Contains(text, "2") {
		t.Errorf("AC1 unmet — the MISSING pre-flight message must count the inherited defects (2) so an operator knows the size of the gap without counting per-id lines. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionPreflightIncompleteFileIsNamed — NEGATIVE. The
// disposition file EXISTS but covers only one of the two inherited ids. The
// pre-flight must name the gap as INCOMPLETE (not MISSING — the operator
// action differs: finish the file, not author it from scratch) and name
// which id(s) are uncovered.
func TestClassify_DispositionPreflightIncompleteFileIsNamed(t *testing.T) {
	ws, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
			// d2 deliberately absent — the file exists but is short.
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Fatalf("fixture is wrong: d2 is unaccounted for and must block PASS")
	}
	text := diagsText(diags)
	if !strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("AC2 unmet — diagnostics must name the structural pre-flight finding %q when defect-dispositions.json covers fewer ids than the ancestor ledger enumerates. diagnostics:\n%s", dispositionPreflightIncomplete, text)
	}
	if !strings.Contains(text, "d2") {
		t.Errorf("AC2 unmet — the INCOMPLETE pre-flight message must name which id(s) are uncovered (d2), not merely a count. diagnostics:\n%s", text)
	}
	if strings.Contains(text, dispositionPreflightMissing) {
		t.Errorf("a partially-covered file must be reported INCOMPLETE, never MISSING — the two markers describe different operator actions. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionPreflightCompleteFileNoFalsePositive — EDGE, the
// anti-no-op half of Task 3. A pre-flight that fires on every continuation
// regardless of completeness proves nothing. Both inherited ids are FIXED
// with resolvable evidence: the cycle must PASS and neither new marker may
// appear.
func TestClassify_DispositionPreflightCompleteFileNoFalsePositive(t *testing.T) {
	ws, req := continuationFixture(t, 1330, 1342, []string{
		"boundary refresh does not repin the short sha",
		"symlinked test-suffix bypasses probe quarantine",
	})
	cite1 := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	cite2 := evidenceFile(t, req.ProjectRoot, "go/internal/core/cyclerun.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite1, "reason": "landed"},
			map[string]any{"id": "d2", "status": "FIXED", "evidence": cite2, "reason": "landed"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("a fully-dispositioned continuation must PASS; verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
	text := diagsText(diags)
	if strings.Contains(text, dispositionPreflightMissing) || strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("a complete defect-dispositions.json must trip NEITHER new pre-flight marker — a pre-flight that always fires proves nothing. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionPreflightNoAncestorNoOp — EDGE, the anti-no-op
// half for ORDINARY cycles. A cycle with no continuation manifest and no
// ancestor ledger (the overwhelming majority of cycles) must never see
// either new marker, regardless of whether it happens to have a
// defect-dispositions.json lying around.
func TestClassify_DispositionPreflightNoAncestorNoOp(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	req := core.PhaseRequest{Cycle: 1342, Workspace: ws, ProjectRoot: t.TempDir()}

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("an ordinary, non-continuation cycle must PASS; verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
	text := diagsText(diags)
	if strings.Contains(text, dispositionPreflightMissing) || strings.Contains(text, dispositionPreflightIncomplete) {
		t.Errorf("an ordinary cycle with no ancestor ledger must never trip the disposition pre-flight — it has nothing inherited to be incomplete about. diagnostics:\n%s", text)
	}
}
