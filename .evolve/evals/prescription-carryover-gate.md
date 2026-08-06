---
score_cap:
  - criterion: "an OPEN PRESCRIPTION: defect-ledger entry merges into state.CarryoverTodos"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/core/... -run TestMergeWorkspacePrescriptionCarryover_OpenPrescriptionEntryIsCarriedOver -v"
  - criterion: "FIXED and DEFERRED prescription entries are never re-surfaced as carryover"
    max_if_missing: 5
    evidence: "cd go && go test ./internal/core/... -run TestMergeWorkspacePrescriptionCarryover_FixedAndDeferredEntriesAreNotCarriedOver -v"
  - criterion: "the merge is wired into the real cycle-terminal path (finalizeCycle), not merely defined and unwired"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/core/... -run TestRunCycle_MergesPrescriptionCarryoverIntoState -v"
---

# Eval: WARN-shipped audit prescriptions must reach the mandatory carryover flow

> Pins the class fix behind F3 (batch-integrity-review-2026-08-04.md): audit's
> `emitDefectLedger` already tags a WARN-carried structured prescription as an
> OPEN `"PRESCRIPTION: "` row in `<workspace>/defect-ledger.json`
> (`defect_ledger.go:134-198`), and `reconcileAgainstAncestor` already blocks
> PASS on any unaccounted OPEN row — but arming is scoped to formally-bound
> continuations, so an ordinary next-lane ship (the common case) never
> reconciles and the prescription is silently dropped. cycle-1258's own
> prescription (see `repair-crosspoll-debounce-eval.md`) is the live instance
> this class gap produced. The fix mirrors `MergeWorkspaceCarryover`
> (`carryover_merge.go`, the chronicle-s4 pattern): a cycle-terminal hook that
> reads OPEN `PRESCRIPTION:`-tagged ledger rows and merges them into
> `state.CarryoverTodos`, so every future WARN prescription reaches the next
> scout through the already-mandatory carryoverTodos flow. Source incident:
> cycle-1258 WARN-ship (2026 batch), diagnosed live in cycle 1375's scout.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dropped prescription | OPEN PRESCRIPTION entry merges into CarryoverTodos | 6/10 | `go test ./internal/core/... -run ...OpenPrescriptionEntryIsCarriedOver` |
| stale re-nag | resolved (FIXED/DEFERRED) prescriptions stay retired | 5/10 | `go test ./internal/core/... -run ...FixedAndDeferredEntriesAreNotCarriedOver` |
| unwired helper | merge is called from the real finalizeCycle terminal path | 7/10 | `go test ./internal/core/... -run TestRunCycle_MergesPrescriptionCarryoverIntoState` |
