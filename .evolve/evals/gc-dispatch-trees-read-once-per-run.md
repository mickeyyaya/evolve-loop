---
score_cap:
  - criterion: "evolve gc reads the dispatch tree records once per run and shares them between the process pass and the tree pass"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestGCDispatchReadOnce_RecordsCreatedAfterTheFirstPassWaitForTheNextRun$' ./cmd/evolve"
  - criterion: "the only-the-tag-proof-applies WARN prints once per tagged process per run in dry-run and apply mode"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestGCDispatchReadOnce_TheTagProofWarningPrintsOncePerTaggedProcessInEveryMode$' ./cmd/evolve"
  - criterion: "a corrupt record still falls back to the tag proof and a tagged process without a record never warns"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestGCDispatchReadOnce_ACorruptRecordStillFallsBackToTheTagProofOnce|TestGCDispatchReadOnce_ATaggedProcessWithoutARecordWarnsNeverAndStillStops)$' ./cmd/evolve"
---

# Eval: gc reads the dispatch tree records once per run

> Pins the run-scoped read of `.evolve/dispatch-trees` in `evolve gc`. Source: the code review of the dispatch-hygiene follow-ups (inbox item gc-dispatch-trees-read-once-per-run, cycle 2671). The process pass and the tree pass each read the directory, and the tag-proof WARN printed once for each proof evaluation.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| snapshot | a record created after the first pass waits for the next run | 6/10 | `TestGCDispatchReadOnce_RecordsCreatedAfterTheFirstPassWaitForTheNextRun` |
| warn-once | one tag-proof WARN per tagged process, dry-run and apply | 6/10 | `TestGCDispatchReadOnce_TheTagProofWarningPrintsOncePerTaggedProcessInEveryMode` |
| fallback | corrupt record keeps the tag proof; absent record stays silent | 5/10 | `TestGCDispatchReadOnce_ACorruptRecordStillFallsBackToTheTagProofOnce` |
