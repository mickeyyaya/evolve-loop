# internal/core/carryover

> The decomposition design is [03-carryover-lifecycle.md](../decomposition/03-carryover-lifecycle.md) (ADR-0103, unit 03). This page keeps the package-level reasons that the code cannot say.

## Purpose

`internal/core/carryover` owns the lifecycle of the `carryoverTodos[]` array in `state.json`. This array is the memory that the planner of the next cycle reads. One `Lifecycle` owns mint admission, the cycle-closeout readers, the ship-time retirement rule and the persist of the state behind two ports.

## Design

- **One admission rule.** `Append` admits a todo in this order. A cross-cycle fingerprint twin only refreshes the expiry of the survivor. An id twin is skipped. Any other todo appends last. A nil state is inert.
- **Defect minting.** `ApplyDefects` mints one P0 todo for each non-blank defect of a failed record. Its ids count non-blank defects only. The action is capped, and the expiry of the record is inherited and never recomputed. Both mint paths go through the one `admit` rule.
- **Fingerprint.** The identity of a todo across cycles is its action with the cycle token folded out, lower-cased and with whitespace collapsed. Without the fold, the same failure class that is minted again in a later cycle gets a new identity each time. In the flood of 2026-08-10, 124 of 254 entries were such re-mints.
- **Expiry compare is lexical.** Every stamp is RFC3339 UTC at second precision, so lexical order is chronological order. An empty stamp never wins. `defaultExpiresAt` (the caller's now plus the backfill TTL of the boot prune) is the one spelling of that rule.
- **`BoundedExpiry(expiresAt, now)` caps a stamp at the carryover TTL** (2026-10-21). It returns the earlier of the stamp and `defaultExpiresAt(now)`. An empty stamp stays empty, so the boot backfill still stamps it. The failure-learning mint uses it. Pinned by `TestBoundedExpiry_KeepsAnEarlierStampAndCapsALaterOneAtTheTTL`.
- **Closeout order.** `Closeout` runs the memo merge, then the prescription merge, then the triage-dropped retirement. The retirement must come last. If it ran before the merges, a merge can bring back the id that triage just dropped (the cycle-1538 reproduction). The caller persists afterwards.
- **Merge rules.** `MergeTodos` unions by id with the disk copy first, and it keeps the disk copy on a shared id. A todo of a concurrent peer is never overwritten. An in-memory refresh of a todo that is already persisted does not land.
- **Failed records.** `MergeFailedRecords` unions by the key cycle, ts, verdict and recordedAt. The disk order is first, and the incoming record wins on a shared key.
- **Workspace merges are id-only unions.** They do no fingerprint suppression. A memo entry whose action has the fingerprint of an existing P0 todo is still appended.
- **One tolerant reader.** `decodeDocument` reads the workspace documents. An absent file gives no result and no signal. A read fault gives `CARRYOVER_WORKSPACE_READ_FAILED`. A decode error gives `CARRYOVER_WORKSPACE_MALFORMED`.
- **Prescription rows.** `MergePrescriptions` carries only OPEN rows that have the `PrescriptionPrefix`, at `PriorityPrescription`, with the prefix kept in the action. FIXED and DEFERRED prescriptions and ordinary defects are left alone, so a resolved prescription never nags again. The ledger wire shape is decoded here, and not in `defectledger`, so that a fault stays a carryover code. The padded status `" OPEN"` is still accepted.
- **Retirement.** `Retire` removes the todos a ship committed, by id and by every cross-cycle fingerprint twin of a committed todo. It returns a new slice. Blank ids retire nothing, and an empty-action committed entry never retires an uncommitted entry with an empty action. `RetireTriageDropped` is id-only and does not read the reason. A missing or malformed decision retires nothing, and the survivors keep their order.
- **Persist.** When the store offers an `Updater`, only `FailedAt` and `CarryoverTodos` merge into the disk copy, and `LastCycleNumber` stays untouched. Otherwise the whole state goes through `WriteState`. A failure is one `CARRYOVER_PERSIST_FAILED` WARN that names the step, stamped with `LastCycleNumber` as the event cycle, and the in-memory state stands. A nil state is inert. A nil store is a programming error and panics, because the store of the orchestrator is never nil.
- **Signal Center accessor.** The unit holds only an accessor, and it reads the accessor at every use. The Center of the orchestrator is an option that tests apply after construction. A nil accessor, or one that returns nil, is the Null Object. Production roots always wire a Center, and `SignalsWired` proves it.
- **Three caps.** `CapRunes` ends with the ellipsis rune and projects `textcap.CapRunes`. `TruncateRunes` trims and marks with ` …[truncated]` and projects `textcap.TruncateRunes`. `Summary` caps at `MaxSummaryRunes` with the marker ` ...[truncated]`. The three markers differ on purpose, and one file lists the three rules.
- **Priority vocabulary.** The priorities of the producers are one table. `P0` is a failure that blocked a cycle. `P1` is a reasoned objection of a judgment phase. `high` is an unenforced audit prescription. `medium` is a memo todo with no priority.
- **Priority consumer.** `carryoverPriorityRank` in `core` reads the table.
- **Memo shape.** `memoTodo` is the shape that evolve-memo and the retro path write to `<workspace>/carryover-todos.json`. The decoder drops `evidence_pointer`.
- **Open follow-up.** `triageDroppedIDs` is one of three readers of `dropped[]` (`inboxmover.ClosedDroppedIDs` and `committedset.DispositionsFrom` are the others). To unify them behind one declaration in the `committedset` leaf is the follow-up `decision-document-single-declaration`.

## Invariants

- **Closeout retires last.** Pinned by the closeout-order tests in `lifecycle_test.go`.
- **`ApplyDefects` on a nil state panics.** That is the contract before the extraction. Only `Append` guards nil, so one admission rule cannot quietly widen it. Pinned in `lifecycle_test.go`.
- **The caps are projections.** `textcap_projection_test.go` pins that neither rule re-implements `textcap`. `defectledger_projection_test.go` pins that `PrescriptionPrefix` is the prefix of the ledger.

## Findings

- None.
