package audit

// closure_claim_demotion_test.go — RED contract for the cycle-1502
// verdict-incoherence halt (batch-20260817a): the closure-citation gate forced
// FAIL over ONE summary line lacking a same-line citation, on a report whose
// per-id defect-dispositions covered EVERY inherited defect and whose
// continuation defect-ledger reconcile had verified that accounting (acs
// 165/0, 8/8 predicates, all 4 ids dispositioned). The reconcile's verified
// machine record is strictly stronger evidence than the line citation the
// prose gate demands — so when the reconcile RAN against a lineage and
// accounted every defect, prose closure-misses demote to warning diagnostics
// instead of verdict-forcing FAIL. Every other path is unchanged: a blocked
// reconcile still forces, and a NON-continuation cycle (no lineage, no
// dispositions — the original 1255 laundering shape) still forces.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// The cycle-1502 line-139 shape: a WARN summary asserting closure, no
// citation on that line.
const demotionClosureReport = "# Audit Report\n\n## Findings\n\n" +
	"the cycle-1490 defect is verified closed\n\n## Verdict\n**WARN**\n"

func TestClassify_ClosureMissDemotedWhenLineageFullyAccounted(t *testing.T) {
	ws, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(demotionClosureReport, req, core.BridgeResponse{})
	if verdict == core.VerdictFAIL {
		t.Fatalf("closure-claim line-citation miss forced FAIL although the defect-ledger reconcile verified every inherited defect against its per-id disposition — the machine record outranks prose formatting (cycle-1502).\ndiagnostics:\n%s", diagsText(diags))
	}
	text := diagsText(diags)
	if !strings.Contains(text, "closure claim") {
		t.Errorf("the demoted miss must still surface as a diagnostic (the formatting note is not waived, only its verdict force). diagnostics:\n%s", text)
	}
}

func TestClassify_ClosureMissOutsideLineageStillForces(t *testing.T) {
	// The leak guard: an ACCOUNTED lineage must not vouch for a claim about an
	// UNRELATED cycle — no record here covers cycle-900.
	ws, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
		},
	})
	unrelated := "# Audit Report\n\n## Findings\n\n" +
		"the cycle-900 defect is verified closed\n\n## Verdict\n**WARN**\n"
	verdict, _, _ := hooks{}.Classify(unrelated, req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("a closure claim about a cycle OUTSIDE the accounted lineage must keep forcing FAIL; got %s", verdict)
	}
}

func TestClassify_RefLessStrongClaimStillForcesOnAccountedLineage(t *testing.T) {
	// Review BLOCK-1: a ref-less "verified closed" — the canonical laundering
	// sentence — names no cycle the machine record could vouch for; it must
	// keep the full gate even when this lane's own lineage is accounted.
	ws, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
		},
	})
	refless := "# Audit Report\n\n## Findings\n\n" +
		"the inherited CRITICAL is verified closed\n\n## Verdict\n**WARN**\n"
	verdict, _, _ := hooks{}.Classify(refless, req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("a ref-less strong-rung closure claim must keep forcing FAIL on an accounted lineage (the strong rung is never guard-suppressed); got %s", verdict)
	}
}

func TestClassify_MissingAncestorLedgerDoesNotVouchLineage(t *testing.T) {
	// Review BLOCK-2: an unblocked reconcile whose ancestor ledger is ABSENT
	// verified nothing — the closure gate is the backstop that makes a deleted
	// ancestor ledger non-silent, and it must keep forcing.
	ws, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	if err := os.Remove(filepath.Join(req.ProjectRoot, ".evolve", "runs", "cycle-1490", ledgerFile)); err != nil {
		t.Fatalf("remove ancestor ledger: %v", err)
	}
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite, "reason": "landed"},
		},
	})
	verdict, _, _ := hooks{}.Classify(demotionClosureReport, req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("a missing ancestor ledger must not vouch the lineage — the closure gate is the deleted-ledger backstop; got %s", verdict)
	}
}

func TestClassify_ClosureMissStillForcesWhenReconcileBlocked(t *testing.T) {
	// Same report, dispositions ABSENT: the reconcile blocks, nothing verified
	// the closure claims — the gate must keep forcing FAIL.
	_, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	verdict, _, _ := hooks{}.Classify(demotionClosureReport, req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("blocked reconcile must keep the closure gate forcing FAIL; got %s", verdict)
	}
}

func TestClassify_ClosureMissStillForcesOnNonContinuation(t *testing.T) {
	// No lineage at all — the original 1255 laundering shape: an ordinary
	// cycle asserting a prior cycle's defect closed with no record anywhere.
	verdict, _ := classifyWith(t, demotionClosureReport, func(ws string) {
		yes := true
		writeACSVerdictShip(t, ws, 0, &yes)
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("non-continuation closure claim without citation must keep forcing FAIL (1255 shape); got %s", verdict)
	}
}
