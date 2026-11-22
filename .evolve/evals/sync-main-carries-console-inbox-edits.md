---
score_cap:
  - criterion: "sync-main carries a console route-console edit (route, routed_*, deps blocker) through the merge"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge$' ./cmd/evolve"
  - criterion: "sync-main carries a console consume move into inbox/consumed and still refuses a move that rewrites an authored field"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestSyncMain_CarriesAConsoleConsumeMove|TestSyncMain_AConsumeMoveThatAlsoRewritesAnAuthoredFieldStillRefuses)$' ./cmd/evolve"
  - criterion: "sync-main removes an untracked copy identical to origin and keeps a differing copy"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestSyncMain_RemovesUntrackedCopiesIdenticalToOrigin$' ./cmd/evolve"
  - criterion: "A same-field conflict keeps origin's value and names the dropped console edit"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestSyncMain_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit$' ./cmd/evolve"
  - criterion: "A hand edit of an authored field still refuses the sync"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses$' ./cmd/evolve"
  - criterion: "Every top-level test of cmd/evolve calls t.Parallel or is listed serial, the untracked-copy helper tests included"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial$' ./cmd/evolve"
---

# Eval: sync-main carries console inbox edits

> Pins the sync-main contract of cycle 2583. On 2026-11-19 and 2026-11-20 a console route-console, edit, consume or withdraw on a tracked inbox file made `evolve sync-main` refuse, so `wave next` waited about one hour for a separate inbox PR at each boundary. The console edit now rides through the merge, origin wins a same-field conflict, and an authored-field hand edit still refuses.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| route-edit | console route and blocker edit lands | 4/10 | `TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge` |
| consume-move | consume move lands, authored rewrite refuses | 5/10 | `TestSyncMain_CarriesAConsoleConsumeMove` |
| untracked-copy | identical copy removed, differing copy kept | 5/10 | `TestSyncMain_RemovesUntrackedCopiesIdenticalToOrigin` |
| conflict | origin wins, dropped edit named | 5/10 | `TestSyncMain_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit` |
| preservation | authored-field hand edit refuses | 6/10 | `TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses` |
| parallel-default | helper tests of the untracked-copy removal run in parallel (cycle 2597, continuation of the cycle 2583 failure) | 6/10 | `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial` |
