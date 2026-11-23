---
score_cap:
  - criterion: "A batch that closed an inbox item is followed by a sync-main that passes, with the item gone from the tracked inbox root"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestJourneyInboxClosureBatchThenSyncMainPasses$' ./cmd/evolve"
  - criterion: "sync-main still refuses a tracked deletion that is no closure (an unclosed inbox item, a file outside the inbox)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestJourneyInboxClosureStillRefusesOtherTrackedDeletions$' ./cmd/evolve"
  - criterion: "A batch that closed nothing leaves local main at origin main and sync-main unchanged"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestJourneyInboxClosureWithNoClosedItemLeavesSyncMainUnchanged$' ./cmd/evolve"
---

# Eval: integration-batch-inbox-closures-reach-main

> Pins the journey of cycle 2611: `evolve integration batch` lands PRs that close tracked inbox items, then `evolve sync-main` (the step the wave boundary runs) returns 0 with no manual ship. The source incident is the boundary 187 to 188 stop: the batch moved three tracked items into the ignored `inbox/processed` folder of the checkout only, and `sync-main` refused the three tracked deletions with rc 3.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| closure-then-sync | The journey passes and the item leaves the tracked root | 3/10 | `go test -run TestJourneyInboxClosureBatchThenSyncMainPasses ./cmd/evolve` |
| other-deletions-refused | A non-closure tracked deletion is still refused (rc 3, path named) | 5/10 | `go test -run TestJourneyInboxClosureStillRefusesOtherTrackedDeletions ./cmd/evolve` |
| nothing-closed | Zero closed items leave sync-main unchanged | 6/10 | `go test -run TestJourneyInboxClosureWithNoClosedItemLeavesSyncMainUnchanged ./cmd/evolve` |
