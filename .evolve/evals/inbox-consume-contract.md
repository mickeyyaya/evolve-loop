---
score_cap:
  - criterion: "ClosesInboxIDs ignores Closes-Inbox lines inside fenced code blocks and lines that are not at column 0"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestClosesInboxIDs$' ./internal/inboxmover"
  - criterion: "a quoted or fenced Closes-Inbox marker in a build report consumes no inbox item at ship, and a genuine column-0 marker still does"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestPromoteInbox_(QuotedClosesInboxMarkerConsumesNothing|ColumnZeroMarkerAfterFenceStillConsumes)$' ./internal/phases/ship"
  - criterion: "ConsumedRecord is single-sourced: every key of the tracked consumed corpus parses into it and round-trips"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestConsumedRecord_RoundTripAgainstCorpus|TestParseConsumedRecord_Shapes)$' ./internal/inboxmover"
  - criterion: "the continuation stamp written on release carries no host path"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestMover_Release_ContinuationStampRedactsHostPaths$' ./internal/inboxmover/lifecycle"
---

# Eval: inbox consume contract sweep

> Pins the open prescriptions of the inbox item `inbox-consume-contract`
> (cycle 2118): quoted upstream text must not forge a Closes-Inbox closure
> claim, the consumed record has one Go type that the tracked corpus
> round-trips through, and the release continuation stamp keeps host paths
> out of the shared inbox. The host-path criterion was already satisfied
> through `continuation.ReadManifest`; the test keeps it satisfied.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| forged-closure | fenced and indented markers claim nothing | 7/10 | `go test -run '^TestClosesInboxIDs$' ./internal/inboxmover` |
| ship-caller | the ship promotion path consumes nothing from quoted text | 7/10 | `go test -run '^TestPromoteInbox_(Quoted...|ColumnZero...)$' ./internal/phases/ship` |
| single-source | ConsumedRecord covers the corpus | 6/10 | `go test -run '^(TestConsumedRecord_RoundTripAgainstCorpus|TestParseConsumedRecord_Shapes)$' ./internal/inboxmover` |
| host-path | release stamp is tilde-redacted | 6/10 | `go test -run '^TestMover_Release_ContinuationStampRedactsHostPaths$' ./internal/inboxmover/lifecycle` |
