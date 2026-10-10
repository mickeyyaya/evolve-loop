package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
	_, req := continuationFixture(t, 1490, 1502, []string{
		"retirement region uncovered",
	})
	verdict, _, _ := hooks{}.Classify(demotionClosureReport, req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("blocked reconcile must keep the closure gate forcing FAIL; got %s", verdict)
	}
}

func TestClassify_ClosureMissStillForcesOnNonContinuation(t *testing.T) {
	verdict, _ := classifyWith(t, demotionClosureReport, func(ws string) {
		yes := true
		writeACSVerdictShip(t, ws, 0, &yes)
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("non-continuation closure claim without citation must keep forcing FAIL (1255 shape); got %s", verdict)
	}
}
