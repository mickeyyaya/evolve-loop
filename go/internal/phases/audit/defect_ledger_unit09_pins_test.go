package audit

// defect_ledger_unit09_pins_test.go — ADR-0103 unit 09, step 1: the pre-move
// pins on the host's order and the gate's hidden couplings, written and green
// on 8e8f080f BEFORE the ledger moved into internal/core/defectledger, each
// proven red against its named mutant (design §6 tests 1-5). Every case drives
// the production seam hooks.Classify.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func ledgerDiagnostics(diags []core.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if strings.HasPrefix(d.Message, "defect ledger: ") {
			out = append(out, d.Message)
		}
	}
	return out
}

// Test 1 — the recorded ceiling (defect_ledger.go's laneRegistryBinding doc):
// a manifest-less workspace whose lane-scope pin is garbage arms NOTHING from
// the registry, even though the registry binds the lane. The coupling to
// core.LaneScopeIDs is pinned BEFORE it goes behind dependency injection.
// Mutant: laneRegistryBinding consults the registry under a hard-coded id.
func TestClassify_LaneScopeMalformedDisarmsTheRegistryFallback(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.Remove(filepath.Join(ws, "continuation-manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Fatalf("a destroyed lane-scope pin is the documented ceiling — no lineage, no gate: verdict %q\n%s", verdict, diagsText(diags))
	}
	if got := ledgerDiagnostics(diags); len(got) != 0 {
		t.Fatalf("no ledger diagnostic without a lane identity: %v", got)
	}
}

// Test 2 — Classify's order: reconcile (the merge write-back) runs BEFORE emit
// (this cycle's own rejection), so the written ledger reads current rows,
// then the inherited rows, then the rows this cycle raised.
// Mutant: finalize (emit) hoisted above reconcileContinuation in Classify.
func TestClassify_ReconcilePrecedesEmit_OwnDefectsAppendAfterInherited(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, laundered)
	writeJSON(t, filepath.Join(ws, ledgerFile), map[string]any{
		"origin_cycle": 1270,
		"entries":      []any{map[string]any{"id": "own-first", "text": "already in this workspace", "status": "OPEN"}},
	})
	fixed := evidenceFile(t, req.ProjectRoot, "go/fixed.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{"dispositions": []any{
		map[string]any{"id": "d1", "status": "FIXED", "evidence": fixed},
		map[string]any{"id": "d2", "status": "DEFERRED", "reason": "later"},
		map[string]any{"id": "d3", "status": "DEFERRED", "reason": "later"},
	}})
	verdict, _, _ := hooks{}.Classify(failingReportWithDefects("new defect one", "new defect two"), req, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("a rejecting continuation: %q", verdict)
	}
	var order []string
	for _, e := range readLedger(t, ws).Entries {
		order = append(order, e.Text)
	}
	want := []string{"already in this workspace", laundered[0], laundered[1], laundered[2], "new defect one", "new defect two"}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Fatalf("row order = %q, want current, inherited, then this cycle's new rows", order)
	}
}

// Test 3 — finalize's order: the ledger is emitted BEFORE the predicate
// evidence is sealed, so the seal covers the final ledger state.
// Mutant: the seal hoisted above the emit in finalize.
func TestFinalize_EmitsTheLedgerBeforeTheSealCoversIt(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("# Audit Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	present := false
	h := hooks{
		genVerdict: func(req core.PhaseRequest) error {
			yes := true
			writeACSVerdictShip(t, req.Workspace, 0, &yes)
			return nil
		},
		predicateEvidence: func(req core.PhaseRequest) (func() error, error) {
			return func() error {
				_, err := os.Stat(filepath.Join(req.Workspace, ledgerFile))
				present = err == nil
				return nil
			}, nil
		},
	}
	verdict, diags, _ := h.Classify(failingReportWithDefects("a defect"), core.PhaseRequest{Cycle: 1, Workspace: ws, ProjectRoot: t.TempDir()}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict %q\n%s", verdict, diagsText(diags))
	}
	if !present {
		t.Fatal("the sealer ran before the ledger was emitted — the seal must cover the final ledger state")
	}
}

// Test 4 — arming order: a corrupt manifest blocks with the manifest
// diagnostic BEFORE the registry is consulted; the registry-binding finding
// never fires beside it. Mutant: the registry consulted before the manifest
// error is examined.
func TestClassify_CorruptManifestBlocksEvenWithAHealthyRegistry(t *testing.T) {
	ws, req := reproContinuationFixture(t, 1255, 1285, laundered)
	if err := os.WriteFile(filepath.Join(ws, "continuation-manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if verdict == core.VerdictPASS {
		t.Fatalf("a corrupt manifest must block:\n%s", diagsText(diags))
	}
	got := ledgerDiagnostics(diags)
	if len(got) != 1 || !strings.Contains(got[0], "continuation manifest is unreadable") {
		t.Fatalf("exactly the manifest diagnostic: %v", got)
	}
}

// Test 5 — the merge index: the FIRST current row wins on a duplicated id; a
// later duplicate is neither the index target nor rewritten.
// Mutant: the duplicate `continue` in the index loop removed.
func TestClassify_FirstRowWinsOnDuplicateIds(t *testing.T) {
	ws, req := continuationFixture(t, 1255, 1270, []string{"text A"})
	writeJSON(t, filepath.Join(ws, ledgerFile), map[string]any{
		"origin_cycle": 1255,
		"entries": []any{
			map[string]any{"id": "d1", "text": "text A", "status": "OPEN"},
			map[string]any{"id": "d1", "text": "text B", "status": "OPEN"},
		},
	})
	fixed := evidenceFile(t, req.ProjectRoot, "go/fixed.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{"dispositions": []any{
		map[string]any{"id": "d1", "status": "FIXED", "evidence": fixed},
	}})
	verdict, diags, _ := hooks{}.Classify(passingReport(), req, core.BridgeResponse{})
	if strings.Contains(diagsText(diags), "shadow") || verdict != core.VerdictPASS {
		t.Fatalf("the first row is the index target — no shadow: %q\n%s", verdict, diagsText(diags))
	}
	rows := readLedger(t, ws).Entries
	if len(rows) != 2 || rows[0].Text != "text A" || rows[0].Status != "FIXED" || rows[1].Text != "text B" || rows[1].Status != "OPEN" {
		t.Fatalf("row 1 graded, row 2 untouched: %+v", rows)
	}
}
