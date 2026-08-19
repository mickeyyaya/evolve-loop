---
score_cap:
  - criterion: "The retired inbox item is out of the live backlog — the production loader (.evolve/inbox/*.json) no longer returns completion-contract-cancel-parity"
    max_if_missing: 6
    evidence: "test ! -e .evolve/inbox/2026-07-16T10-30-00Z-completion-contract-cancel-parity.json"
  - criterion: "The closure is recorded in the consumed corpus with an evidence-citing resolution, not deleted silently"
    max_if_missing: 7
    evidence: "grep -q 'completion_cancel_parity_test.go' .evolve/inbox/consumed/2026-07-16T10-30-00Z-completion-contract-cancel-parity.json"
  - criterion: "The closing evidence stays true: the four cancel/completion parity + non-regression tests are green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout|TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout|TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout|TestArtifactDetector_CtxCancelledShortCircuitsDebounce' ./internal/bridge"
---

# Eval: Close the stale completion-contract-cancel-parity inbox item

> Pins the closure of inbox item `completion-contract-cancel-parity` (filed
> 2026-07-16, weight 0.55) as *not-observed / already-fixed*. The defect it
> scoped — stdout/git completion contracts failing closed on an already-dead
> context and mislabelling a benign cancel as `ExitArtifactTimeout` — was
> generalized away by `withFinalPoll` + the explicit `finalPollCtxKey` finality
> signal in `go/internal/bridge/completion.go`, and is pinned by
> `go/internal/bridge/completion_cancel_parity_test.go`. Source incident:
> cycle-1529 scout found the item stale on its own acceptance criterion
> ("close as not-observed if none"), after the item had survived triage sweeps
> since 2026-07-16.
>
> Two durable obligations survive this cycle. First, an item closed on the
> strength of a test suite is only correctly closed while that suite is green —
> so the parity tests are score-capping evidence, not a one-time check. Second,
> the closure must remain a RECORD (consumed corpus, resolution citing the
> proof), never a silent deletion: a deleted item cannot be re-opened when the
> evidence changes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| backlog-retired | Item no longer drawable from `.evolve/inbox/` | 6/10 | `test ! -e .evolve/inbox/2026-07-16T10-30-00Z-...json` |
| closure-recorded | Consumed record cites the closing test file | 7/10 | `grep -q 'completion_cancel_parity_test.go' .evolve/inbox/consumed/...json` |
| evidence-still-true | 4 cancel/completion parity tests green | 8/10 | `go test -run '…CancelAfter…|…CtxCancelledShortCircuits…' ./internal/bridge` |
