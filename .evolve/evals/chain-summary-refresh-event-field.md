---
score_cap:
  - criterion: "a successful cycle-start catalog refresh appends exactly one catalog_refresh ledger entry stamped ok, carrying the resolved refresh_stage"
    max_if_missing: 8
    evidence: "cd go && go test -run TestOrchestrator_CatalogRefresh_LedgerStampsOkOutcome -v ./internal/core/"
  - criterion: "a failed refresh appends a catalog_refresh entry stamped failed without failing the cycle (best-effort contract preserved)"
    max_if_missing: 8
    evidence: "cd go && go test -run TestOrchestrator_CatalogRefresh_LedgerStampsFailedOutcome -v ./internal/core/"
  - criterion: "a nil catalogRefresh (no refresher wired) appends zero catalog_refresh entries"
    max_if_missing: 6
    evidence: "cd go && go test -run TestOrchestrator_CatalogRefresh_NilRefresherNoLedgerEntry -v ./internal/core/"
  - criterion: "the refresh_stage accessor is optional — its absence still stamps the outcome, with an empty stage rather than a fabricated one"
    max_if_missing: 4
    evidence: "cd go && go test -run TestOrchestrator_CatalogRefresh_NoStageAccessorLeavesStageEmpty -v ./internal/core/"
---

# Eval: Ledger entry for the cycle-start model-catalog refresh outcome

> Pins chain-summary-refresh-event-field (cycle-1353). The cycle-start
> live-model-catalog refresh (`go/internal/core/cyclerun.go:586-593`,
> `planCycle`) is fire-and-forget: on failure it only
> `fmt.Fprintf(os.Stderr, ...)` a WARN, and on success it leaves no trace at
> all. Six lines below it, the sibling `operator_directives` refresh
> (`cyclerun.go:599-613`) does the correct thing — it appends a
> `LedgerEntry{Kind: "operator_directives", Action: directivesSet.Version}`
> so the outcome is queryable/auditable. The `catalog.refresh_stage=shadow`
> soak (memory: `model_latest_selection`, "REMAINING: operator
> `refresh_stage:"shadow"` flip at boundary + ≥10-cycle soak") needs exactly
> this kind of audit trail to build confidence toward flipping to
> `enforce` — stderr scrollback is not queryable. This eval pins a new
> `catalog_refresh` ledger entry (Action=outcome ok/failed, Message=resolved
> `refresh_stage` via the new optional `WithCatalogRefreshStage` accessor),
> mirroring the `operator_directives` append pattern exactly.
>
> Scope note: the task description names a third outcome, "skipped" — the
> injected refresher's contract is `func(ctx context.Context) error`, which
> carries no signal distinguishing an internal TTL-skip from a genuine
> success (both return nil). Materializing a "skipped" outcome would require
> widening that closure's contract, which is out of scope for this S-sized
> task; the AC-Materialization table below dispositions it
> `unverifiable-remove` with this rationale.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ok-outcome | Success appends 1 entry, Action=ok, Message=resolved stage | 8/10 | `go test -run TestOrchestrator_CatalogRefresh_LedgerStampsOkOutcome` |
| failed-outcome | Failure appends 1 entry, Action=failed, cycle still PASSes | 8/10 | `go test -run TestOrchestrator_CatalogRefresh_LedgerStampsFailedOutcome` |
| nil-refresher | No refresher wired ⇒ zero entries | 6/10 | `go test -run TestOrchestrator_CatalogRefresh_NilRefresherNoLedgerEntry` |
| optional-stage | No stage accessor ⇒ entry still stamped, empty Message | 4/10 | `go test -run TestOrchestrator_CatalogRefresh_NoStageAccessorLeavesStageEmpty` |
