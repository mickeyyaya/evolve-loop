package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
)

const prescriptionTagPrefix = carryover.PrescriptionPrefix

func warnReportWithPrescription(prescriptions ...string) string {
	p, _ := json.Marshal(prescriptions)
	return "# Audit Report\n\n## Verdict\n**WARN**\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,` +
		`"failure":{"class":"risk-foreseen","defects":[],"prescription":` + string(p) + `}} -->` + "\n"
}

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
