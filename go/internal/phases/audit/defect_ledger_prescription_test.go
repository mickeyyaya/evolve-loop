package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_prescription_test.go — RED contract for cycle-1327's
// `audit-warn-prescription-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91).
//
// Reuse, not a parallel mechanism: emitDefectLedger already fires on WARN
// (audit.go:395) and reconcileAgainstAncestor is already generic over "an OPEN
// entry with an id and text" (defect_ledger.go:322-367). This file pins the
// ONE missing step — emitDefectLedger must also source
// Failure.Prescription — plus proves the existing reconcile/evidence gates
// apply unmodified to a prescription-sourced entry, and that an ordinary,
// prescription-less WARN is byte-for-byte unchanged (the regression guard
// against widening the ledger trigger into every narrative WARN).
//
// Prescription-sourced text carries a "PRESCRIPTION: " prefix (scout report
// Hypothesis 2) so an operator reading defect-ledger.json can distinguish "what
// was wrong" (a defect) from "a foreseen risk's named fix" (a prescription)
// without a second ledger or a schema-breaking Kind field.

const prescriptionTagPrefix = "PRESCRIPTION: "

// warnReportWithPrescription renders an audit-report.md whose evolve-verdict
// sentinel is WARN, carries the given prescription strings and zero defects —
// the exact cycle-1258 shape (a foreseen risk, not a defect) that
// emitDefectLedger currently drops on the floor.
func warnReportWithPrescription(prescriptions ...string) string {
	p, _ := json.Marshal(prescriptions)
	return "# Audit Report\n\n## Verdict\n**WARN**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,` +
		`"failure":{"class":"risk-foreseen","defects":[],"prescription":` + string(p) + `}} -->` + "\n"
}

// -- Criterion 2: a WARN prescription mints an addressable, blocking entry --

// TestDefectLedger_WarnPrescription — positive: a WARN whose sentinel carries
// one prescription and zero defects must still emit defect-ledger.json with
// one OPEN, tagged entry. Negative: an EMPTY prescription array (and empty
// defects) mints nothing — the fix must not make every narrative WARN mint a
// vacuous entry (matches the existing "no structured content -> no ledger"
// rule emitDefectLedger already enforces for defects).
func TestDefectLedger_WarnPrescription(t *testing.T) {
	t.Run("nonempty_prescription_mints_open_entry", func(t *testing.T) {
		ws := t.TempDir()
		yes := true
		writeACSVerdictShip(t, ws, 0, &yes)

		text := "run `git add -f X` or dropIgnoredPaths will silently drop it"
		verdict, _, _ := hooks{}.Classify(
			warnReportWithPrescription(text),
			core.PhaseRequest{Cycle: 1327, Workspace: ws, ProjectRoot: t.TempDir()},
			core.BridgeResponse{},
		)
		if verdict != core.VerdictWARN {
			t.Fatalf("fixture must WARN: verdict = %q, want WARN", verdict)
		}

		doc := readLedger(t, ws)
		if len(doc.Entries) != 1 {
			t.Fatalf("entries = %d, want 1 (the prescription, sourced despite zero defects)", len(doc.Entries))
		}
		e := doc.Entries[0]
		if e.Status != "OPEN" {
			t.Errorf("status = %q, want OPEN on first emission", e.Status)
		}
		want := prescriptionTagPrefix + text
		if e.Text != want {
			t.Errorf("text = %q, want %q — prescription-sourced entries must be tagged distinguishably from defects", e.Text, want)
		}
		if strings.TrimSpace(e.ID) == "" {
			t.Error("prescription entry has empty id — an unaddressable prescription is exactly the F3 gap")
		}
	})

	t.Run("empty_prescription_mints_nothing", func(t *testing.T) {
		ws := t.TempDir()
		yes := true
		writeACSVerdictShip(t, ws, 0, &yes)

		verdict, _, _ := hooks{}.Classify(
			warnReportWithPrescription(),
			core.PhaseRequest{Cycle: 1327, Workspace: ws, ProjectRoot: t.TempDir()},
			core.BridgeResponse{},
		)
		if verdict != core.VerdictWARN {
			t.Fatalf("fixture must WARN: verdict = %q, want WARN", verdict)
		}
		if _, err := os.Stat(filepath.Join(ws, ledgerFile)); err == nil {
			t.Error("a WARN with an empty prescription array (and empty defects) must not mint a ledger — widening the trigger would make every narrative WARN mint a vacuous entry")
		}
	})
}

// -- Criterion 3: an inherited prescription blocks continuation PASS --------

// prescriptionAncestorFixture builds a project root holding an ancestor cycle
// whose ledger holds ONE OPEN, prescription-tagged entry, plus a current
// workspace stamped as that cycle's continuation. Mirrors continuationFixture
// (defect_ledger_test.go) — the reconcile mechanism must be exactly the one
// FAIL-sourced defects already use, not a parallel path.
func prescriptionAncestorFixture(t *testing.T, ancestorCycle, thisCycle int, prescriptionText string) (string, core.PhaseRequest) {
	t.Helper()
	root := t.TempDir()
	ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(ancestorCycle))

	type entry struct {
		ID     string `json:"id"`
		Text   string `json:"text"`
		Status string `json:"status"`
	}
	writeJSON(t, filepath.Join(ancestorWS, ledgerFile), map[string]any{
		"origin_cycle": ancestorCycle,
		"entries":      []entry{{ID: "d1", Text: prescriptionTagPrefix + prescriptionText, Status: "OPEN"}},
	})

	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	writeJSON(t, filepath.Join(ws, "continuation-manifest.json"), map[string]any{
		"cycle":         ancestorCycle,
		"branch":        "cycle-" + strconv.Itoa(ancestorCycle),
		"snapshot_sha":  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"base_sha":      "cafebabecafebabecafebabecafebabecafebabe",
		"findings_path": filepath.Join(ancestorWS, "audit-fail-reason.json"),
	})
	return ws, core.PhaseRequest{Cycle: thisCycle, Workspace: ws, ProjectRoot: root}
}

// TestReconcile_WarnPrescriptionBlocks — the crux of Criterion 3. An inherited
// OPEN prescription entry with no matching disposition row must block PASS via
// the SAME "unaccounted" diagnostic path FAIL-sourced defects already use.
// Edge case: the same entry disposed FIXED with evidence that resolves to a
// real, repo-relative, non-self file unblocks PASS — mirroring
// evidenceResolves' existing gaming-resistance (an unverifiable "evidence:"x""
// closure must still fail, per the file's existing rules) rather than a looser
// check invented just for prescriptions.
func TestReconcile_WarnPrescriptionBlocks(t *testing.T) {
	const prescriptionText = "run `git add -f X` or dropIgnoredPaths will silently drop it"

	t.Run("unaccounted_prescription_blocks_pass", func(t *testing.T) {
		_, req := prescriptionAncestorFixture(t, 1258, 1327, prescriptionText)

		verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
		if verdict == core.VerdictPASS {
			t.Error("a continuation with an unaccounted inherited prescription must not PASS — this is F3's exact gap")
		}
		text := diagsText(diags)
		if !strings.Contains(text, "d1") {
			t.Errorf("the unaccounted prescription entry must be named by id in a diagnostic; diagnostics were:\n%s", text)
		}
	})

	t.Run("resolved_evidence_unblocks_pass", func(t *testing.T) {
		ws, req := prescriptionAncestorFixture(t, 1258, 1327, prescriptionText)
		writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
			"dispositions": []any{
				map[string]any{"id": "d1", "status": "FIXED", "evidence": evidenceFile(t, req.ProjectRoot, "go/internal/inboxmover/prescription.go")},
			},
		})

		verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
		if verdict != core.VerdictPASS {
			t.Errorf("a fully-dispositioned prescription must unblock PASS, got %q; diagnostics:\n%s", verdict, diagsText(diags))
		}
	})

	t.Run("unverifiable_evidence_still_blocks", func(t *testing.T) {
		// Cheapest gaming fake: evidence:"x" resolves to no file under the
		// project root. evidenceResolves must reject this exactly as it does
		// for FAIL-sourced defects (defect_ledger.go) — a looser check
		// invented just for prescriptions would reopen the unverifiable-
		// closure hole the ledger exists to close.
		ws, req := prescriptionAncestorFixture(t, 1258, 1327, prescriptionText)
		writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
			"dispositions": []any{
				map[string]any{"id": "d1", "status": "FIXED", "evidence": "x"},
			},
		})

		verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
		if verdict == core.VerdictPASS {
			t.Error("an unverifiable evidence:\"x\" closure must not unblock PASS")
		}
		text := diagsText(diags)
		if !strings.Contains(text, "d1") {
			t.Errorf("the unresolved closure must still be named by id; diagnostics were:\n%s", text)
		}
	})
}

// -- Criterion 4: a prescription-less WARN is unchanged --------------------

// TestAudit_WarnWithoutPrescription_NoRegression — an ordinary WARN with no
// `prescription` field (the pre-fix shape, still the overwhelming majority of
// real audits) must behave identically to today: no ledger entry from this
// source, PASS/WARN grading unchanged. Guards the fix against widening the
// ledger trigger into every narrative WARN — the same failure mode
// emitDefectLedger's own doc comment already warns against.
func TestAudit_WarnWithoutPrescription_NoRegression(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)

	verdict, _, _ := hooks{}.Classify(
		narrativeReport("WARN"),
		core.PhaseRequest{Cycle: 1327, Workspace: ws, ProjectRoot: t.TempDir()},
		core.BridgeResponse{},
	)
	if verdict != core.VerdictWARN {
		t.Fatalf("fixture must WARN: verdict = %q, want WARN", verdict)
	}
	if _, err := os.Stat(filepath.Join(ws, ledgerFile)); err == nil {
		t.Error("a prescription-less, defect-less WARN must not mint a ledger — the pre-fix behavior must be unchanged")
	}
}
