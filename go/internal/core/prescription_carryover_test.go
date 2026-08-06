package core

// prescription_carryover_test.go — RED contract for cycle-1375 task
// `prescription-carryover-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91; scout-report.md Task 2, fleet lane
// audit-warn-prescriptions-unenforced).
//
// DEFECT BEING FIXED: audit's emitDefectLedger already tags a WARN-carried
// structured Prescription[] entry as an OPEN "PRESCRIPTION: <text>" row in
// <workspace>/defect-ledger.json (defect_ledger.go:134-198), and
// reconcileAgainstAncestor already blocks PASS on any unaccounted OPEN row —
// but ONLY when the current cycle is formally bound as a continuation of the
// ledger-holding cycle. An ordinary next-lane ship (the common case — triage
// picks lanes by content, not by ledger lineage) never reconciles, so the
// prescription is silently dropped at ship. cycle-1258's own prescription
// ("materialize .evolve/evals/artifact-ready-crosspoll-debounce.md, git add -f
// past .gitignore") is the live, still-unrepaired instance (Task 1, this cycle).
//
// FIX (Builder authors go/internal/core/prescription_carryover.go + wires the
// call site in finalizeCycle beside MergeWorkspaceCarryover, per
// carryover_merge.go:26-40's existing pattern): a cycle-terminal hook that, if
// <workspace>/defect-ledger.json exists, tolerant-decodes its entries, keeps
// only Status=="OPEN" rows whose Text carries the exact "PRESCRIPTION: " prefix
// emitDefectLedger already writes (defect_ledger.go:181), and merges one
// CarryoverTodo per surviving entry into state.CarryoverTodos (dedup by id via
// the existing mergeCarryoverTodos — idempotent on re-entry, same idiom as
// MergeWorkspaceCarryover). This makes every future WARN prescription reach the
// next scout through the already-mandatory carryoverTodos flow, regardless of
// continuation binding.
//
// These tests are authored by the TDD engineer and are RED now (they will not
// even compile until MergeWorkspacePrescriptionCarryover exists — a valid RED
// per the compile-failure rule). The Builder must make them GREEN by adding
// production code ONLY; it must NOT modify this file.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : TestRunCycle_MergesPrescriptionCarryoverIntoState — the
//     real terminal path (finalizeCycle) persists the prescription todo. This is
//     the load-bearing anti-no-op signal: a helper that exists but is never
//     wired into the terminal hook leaves F3's gap open and FAILS here.
//   - Negative        : TestMergeWorkspacePrescriptionCarryover_FixedAndDeferredEntriesAreNotCarriedOver —
//     a FIXED or DEFERRED prescription must NOT surface as a new carryover (it
//     is already resolved; re-surfacing it would nag forever).
//   - Semantic         : TestMergeWorkspacePrescriptionCarryover_NonPrescriptionOpenEntryIsIgnored —
//     an OPEN structured-defect row that is NOT tagged "PRESCRIPTION: " (i.e.
//     the ordinary reconcile-gate defect, already covered by
//     reconcileAgainstAncestor) must be left alone — this hook is scoped to the
//     PRESCRIPTION: subset only, never a blanket ledger-to-carryover mirror.
//   - Edge/OOD         : TestMergeWorkspacePrescriptionCarryover_AbsentLedgerIsNoOp,
//     TestMergeWorkspacePrescriptionCarryover_MalformedLedgerWarnsNotFails — a
//     missing or corrupt ledger must never abort the cycle-terminal hook.
//   - Semantic         : TestMergeWorkspacePrescriptionCarryover_DedupesById — a
//     second finalize over the same still-OPEN ledger must not duplicate the
//     carryover row (crash-resume / double-invocation idempotence).

import (
	"context"
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

// TestMergeWorkspacePrescriptionCarryover_OpenPrescriptionEntryIsCarriedOver —
// the direct AC from scout-report.md Task 2: an OPEN "PRESCRIPTION: ..." row
// merges into state.CarryoverTodos.
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

// TestMergeWorkspacePrescriptionCarryover_FixedAndDeferredEntriesAreNotCarriedOver
// — the negative half of the AC: a resolved prescription must not re-surface.
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

// TestMergeWorkspacePrescriptionCarryover_NonPrescriptionOpenEntryIsIgnored —
// an OPEN entry lacking the "PRESCRIPTION: " prefix is an ordinary structured
// defect already governed by reconcileAgainstAncestor; this hook must not
// double-surface it as a carryover todo.
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

// TestMergeWorkspacePrescriptionCarryover_AbsentLedgerIsNoOp — a workspace with
// no defect-ledger.json (the ordinary PASS cycle) must not panic or fabricate
// todos.
func TestMergeWorkspacePrescriptionCarryover_AbsentLedgerIsNoOp(t *testing.T) {
	ws := t.TempDir() // no defect-ledger.json written
	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC()) // must not panic
	if len(state.CarryoverTodos) != 0 {
		t.Fatalf("RED: absent ledger must be a no-op, got %+v", state.CarryoverTodos)
	}
}

// TestMergeWorkspacePrescriptionCarryover_MalformedLedgerWarnsNotFails — a
// corrupt ledger must be tolerated (no panic, no fatal), mirroring
// MergeWorkspaceCarryover's malformed-file discipline. The cycle-terminal hook
// must never abort the cycle over a malformed anti-laundering record.
func TestMergeWorkspacePrescriptionCarryover_MalformedLedgerWarnsNotFails(t *testing.T) {
	ws := t.TempDir()
	writeDefectLedgerFixture(t, ws, `{ this is not valid json `)
	state := &State{}
	MergeWorkspacePrescriptionCarryover(state, ws, 1375, time.Now().UTC()) // must not panic
	if len(state.CarryoverTodos) != 0 {
		t.Fatalf("RED: malformed ledger produced %d todos, want 0", len(state.CarryoverTodos))
	}
}

// TestMergeWorkspacePrescriptionCarryover_DedupesById — re-entry over the same
// still-OPEN ledger (crash-resume / double-invocation) must not duplicate the
// carried-over row.
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

// TestRunCycle_MergesPrescriptionCarryoverIntoState is the WIRING contract: a
// cycle whose workspace holds an OPEN "PRESCRIPTION: " ledger row must, at the
// cycle-terminal hook (finalizeCycle, the real terminal path — never a helper
// that merely exists unwired), land it in the PERSISTED state. This is the
// caller-proof predicate: without the call site in finalizeCycle, F3's gap
// stays open even though the helper compiles and unit-passes on its own.
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

	if _, err := o.finalizeCycle(context.Background(), cs, 1375, "same-head", "", result, state); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	got := f.mem.st.CarryoverTodos
	if !carryoverTodoExists(got, "d-eval-1258") {
		t.Fatalf("RED: OPEN prescription entry d-eval-1258 not merged into persisted state.CarryoverTodos: %+v\n"+
			"Builder must call MergeWorkspacePrescriptionCarryover(state, cs.WorkspacePath, cycle, now) "+
			"in finalizeCycle, beside the existing MergeWorkspaceCarryover call.", got)
	}
}
