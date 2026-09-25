---
score_cap:
  - criterion: "Every failurelog state writer (Record, PruneExpired, PruneByClassification, PruneExpiredCarryoverTodos, BackfillLegacyCarryoverExpiry, IncrementCarryoverUnpicked) writes THROUGH a symlinked state.json: the link survives and the canonical file carries the write"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestStateWriters_PreserveSymlinkedStatePath$' ./internal/failurelog | grep -q -- '--- PASS: TestStateWriters_PreserveSymlinkedStatePath'"
  - criterion: "failurelog.Record keeps its never-auto-create contract: an absent (or dangling-linked) state.json returns ErrStateMissing and writes nothing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestRecord_StateMissing$' ./internal/failurelog | grep -q -- '--- PASS: TestRecord_StateMissing'"
  - criterion: "The failurelog package carries no regression, including the atomicWriteJSON test seam's existing overrides"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/failurelog"
---

# Eval: failurelog state writers resolve symlinks before tmp+rename

> Scout task 2 of inbox item `statejson-latent-unresolved-writers` (cycle 1690).
> Every exported failurelog state writer ends in `atomicWriteJSON(statePath, …)`,
> a raw tmp+rename. When `statePath` is a worktree link to the canonical state
> file, the rename REPLACES the link with a regular file (the cycle-999 sever),
> and every later mutation strands in the detached copy. There are six such
> sites (record.go:140, prune.go:82/148, prune_carryover.go:70/135/182), not
> three. All six must write through to the resolved target. Source incidents:
> cycle-999/1000; the 2026-07-21 go-reviewer MEDIUM. The authority eval for the
> whole item is `statejson-latent-unresolved-writers.md`.
>
> Re-authored by the cycle-1690 TDD phase. The scout draft (1) was unparseable
> by the eval-quality checker (L1 "zero parsed commands", WARN), and (2) required
> Record to "succeed and materialize the file" through a DANGLING link. That
> contradicts Record's documented contract (preflight owns creating state.json;
> `TestRecord_StateMissing`), so the contract-consistent behavior is pinned
> instead: a dangling link is left untouched and Record returns `ErrStateMissing`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| write-through | all six writers keep the link and land on canonical | 9/10 | `go test -run '^TestStateWriters_PreserveSymlinkedStatePath$' ./internal/failurelog` |
| no-auto-create | Record's ErrStateMissing contract survives | 6/10 | `go test -run '^TestRecord_StateMissing$' ./internal/failurelog` |
| no-regression | failurelog suite green (seam overrides intact) | 5/10 | `go test ./internal/failurelog` |
