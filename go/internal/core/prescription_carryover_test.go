package core

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeDefectLedgerFixture writes a defect-ledger.json (the on-disk wire shape
// emitDefectLedger produces: {"origin_cycle": N, "entries": [...]}) into dir.
func writeDefectLedgerFixture(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "defect-ledger.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write defect-ledger.json: %v", err)
	}
}

const ledgerOnePrescriptionOpen = `{
  "origin_cycle": 1258,
  "entries": [
    {"id": "d-eval-1258", "text": "PRESCRIPTION: materialize .evolve/evals/artifact-ready-crosspoll-debounce.md and git add -f past .gitignore", "status": "OPEN"}
  ]
}`

func TestMergeWorkspacePrescriptionCarryover_OpenPrescriptionEntryIsCarriedOver(t *testing.T) {
	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, ledgerOnePrescriptionOpen)

	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC())

	if !carryoverTodoExists(state.CarryoverTodos, "d-eval-1258") {
		t.Fatalf("RED: OPEN prescription entry d-eval-1258 not merged into state.CarryoverTodos: %+v\n"+
			"Builder must add MergeWorkspacePrescriptionCarryover, called from finalizeCycle "+
			"beside MergeWorkspaceCarryover.", state.CarryoverTodos)
	}
	var got CarryoverTodo
	for _, td := range state.CarryoverTodos {
		if td.ID == "d-eval-1258" {
			got = td
		}
	}
	if !strings.Contains(got.Action, "artifact-ready-crosspoll-debounce.md") {
		t.Fatalf("RED: carried-over Action must retain the prescription text, got %q", got.Action)
	}
	if got.FirstSeenCycle != 1375 {
		t.Fatalf("RED: FirstSeenCycle = %d, want 1375 (the merging cycle)", got.FirstSeenCycle)
	}
	if got.ExpiresAt == "" {
		t.Fatal("RED: ExpiresAt left unstamped — prune can never age this todo out")
	}
	if _, err := time.Parse(time.RFC3339, got.ExpiresAt); err != nil {
		t.Fatalf("RED: ExpiresAt %q is not RFC3339: %v", got.ExpiresAt, err)
	}
}

func TestMergeWorkspacePrescriptionCarryover_FixedAndDeferredEntriesAreNotCarriedOver(t *testing.T) {
	t.Run("FIXED", func(t *testing.T) {
		ws := t.TempDir()
		writeDefectLedgerFixture(t, ws, `{
  "origin_cycle": 1258,
  "entries": [
    {"id": "d-fixed", "text": "PRESCRIPTION: already applied", "status": "FIXED", "evidence": "commit abc123"}
  ]
}`)
		state := &State{}
		MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC())
		if carryoverTodoExists(state.CarryoverTodos, "d-fixed") {
			t.Fatalf("RED: FIXED prescription must NOT be carried over, got %+v", state.CarryoverTodos)
		}
	})

	t.Run("DEFERRED", func(t *testing.T) {
		ws := t.TempDir()
		writeDefectLedgerFixture(t, ws, `{
  "origin_cycle": 1258,
  "entries": [
    {"id": "d-deferred", "text": "PRESCRIPTION: won't fix this cycle", "status": "DEFERRED", "reason": "out of scope"}
  ]
}`)
		state := &State{}
		MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC())
		if carryoverTodoExists(state.CarryoverTodos, "d-deferred") {
			t.Fatalf("RED: DEFERRED prescription must NOT be carried over, got %+v", state.CarryoverTodos)
		}
	})
}

func TestMergeWorkspacePrescriptionCarryover_NonPrescriptionOpenEntryIsIgnored(t *testing.T) {
	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, `{
  "origin_cycle": 1258,
  "entries": [
    {"id": "d-plain", "text": "structured defect: missing error handling", "status": "OPEN"}
  ]
}`)
	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC())
	if carryoverTodoExists(state.CarryoverTodos, "d-plain") {
		t.Fatalf("RED: non-prescription OPEN entry must be ignored by this hook, got %+v", state.CarryoverTodos)
	}
}

func TestMergeWorkspacePrescriptionCarryover_AbsentLedgerIsNoOp(t *testing.T) {
	ws := t.TempDir() // no defect-ledger.json written
	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC()) // must not panic
	if len(state.CarryoverTodos) != 0 {
		t.Fatalf("RED: absent ledger must be a no-op, got %+v", state.CarryoverTodos)
	}
}

func TestMergeWorkspacePrescriptionCarryover_MalformedLedgerWarnsNotFails(t *testing.T) {
	t.Run("wired: the malformed ledger is a CARRYOVER_WORKSPACE_MALFORMED signal", func(t *testing.T) {
		ws := t.TempDir()
		writeDefectLedgerFixture(t, ws, `{ this is not valid json `)
		signals, got := recordingCenter()
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signals))
		state := &State{}
		o.carryover().MergePrescriptions(state, ws, 1375, time.Now().UTC())
		warned := eventsOfKind(*got, signalcenter.KindCarryoverWarning)
		if len(state.CarryoverTodos) != 0 || len(warned) != 1 || warned[0].Code != carryover.CodeWorkspaceMalformed || warned[0].Origin != "Lifecycle.MergePrescriptions" {
			t.Fatalf("the wired lifecycle reports the malformed ledger once and merges nothing: %+v", *got)
		}
	})

	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, `{ this is not valid json `)
	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC()) // must not panic
	if len(state.CarryoverTodos) != 0 {
		t.Fatalf("RED: malformed ledger produced %d todos, want 0", len(state.CarryoverTodos))
	}
}

func TestMergeWorkspacePrescriptionCarryover_DedupesById(t *testing.T) {
	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, ledgerOnePrescriptionOpen)

	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC())
	afterFirst := len(state.CarryoverTodos)
	MergeWorkspacePrescriptionCarryover(state, ws, 1376, time.Now().UTC())

	if afterFirst != 1 {
		t.Fatalf("RED: first merge added %d todos, want 1", afterFirst)
	}
	if got := len(state.CarryoverTodos); got != 1 {
		t.Fatalf("RED: re-entry duplicated the prescription todo: got %d, want 1 (dedup by id must be idempotent): %+v",
			got, state.CarryoverTodos)
	}
}

func TestRunCycle_MergesPrescriptionCarryoverIntoState(t *testing.T) {
	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, ledgerOnePrescriptionOpen)

	f := &fakeUpdaterStorage{}
	o := &Orchestrator{
		storage: f,
		gitHEAD: func() (string, error) { return "same-head", nil },
	}

	cs := CycleState{WorkspacePath: ws}
	result := &CycleResult{FinalVerdict: VerdictWARN}
	state := &State{}

	if _, err := o.finalizeCycle(context.Background(), cs, 1375, "same-head", "", result, state, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	got := f.mem.st.CarryoverTodos
	if !carryoverTodoExists(got, "d-eval-1258") {
		t.Fatalf("RED: OPEN prescription entry d-eval-1258 not merged into persisted state.CarryoverTodos: %+v\n"+
			"Builder must call MergeWorkspacePrescriptionCarryover(state, cs.WorkspacePath, cycle, now) "+
			"in finalizeCycle, beside the existing MergeWorkspaceCarryover call.", got)
	}
}
