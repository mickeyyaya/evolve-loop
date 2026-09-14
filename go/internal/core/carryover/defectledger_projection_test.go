package carryover

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Test 47 (ADR-0103 unit 09) — the consumer pin of the ledger's vocabulary:
// PrescriptionPrefix projects the leaf's; MergePrescriptions over a ledger the
// leaf wrote keeps exactly the OPEN + prefixed rows, skips a FIXED
// prescription and an ordinary defect, and STILL tolerates a padded " OPEN"
// (preserved quirk Q3 — the lifecycle's TrimSpace compare).
func TestCarryover_ProjectsTheLedgerVocabulary(t *testing.T) {
	if PrescriptionPrefix != defectledger.PrescriptionPrefix {
		t.Fatalf("PrescriptionPrefix = %q, the leaf spells %q", PrescriptionPrefix, defectledger.PrescriptionPrefix)
	}
	ws := t.TempDir()
	doc := defectledger.Doc{OriginCycle: 1270, Entries: []defectledger.Entry{
		{ID: "p1", Text: defectledger.PrescriptionPrefix + "add a regression test", Status: defectledger.StatusOpen},
		{ID: "p2", Text: defectledger.PrescriptionPrefix + "already done", Status: defectledger.StatusFixed, Evidence: "go/x.go"},
		{ID: "d1", Text: "an ordinary defect", Status: defectledger.StatusOpen},
		{ID: "p3", Text: defectledger.PrescriptionPrefix + "padded status", Status: " " + defectledger.StatusOpen},
	}}
	if err := defectledger.Write(ws, doc); err != nil {
		t.Fatal(err)
	}
	var state cyclestate.State
	New().MergePrescriptions(&state, ws, 1271, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
	if len(state.CarryoverTodos) != 2 || state.CarryoverTodos[0].ID != "p1" || state.CarryoverTodos[1].ID != "p3" || state.CarryoverTodos[0].Priority != PriorityPrescription {
		t.Fatalf("exactly the OPEN prescriptions, the padded status tolerated: %+v", state.CarryoverTodos)
	}
	if filepath.Base(filepath.Join(ws, defectledger.LedgerFile)) != "defect-ledger.json" {
		t.Fatal("the lifecycle reads the leaf's artifact name")
	}
}
