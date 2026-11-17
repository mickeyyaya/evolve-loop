---
score_cap:
  - criterion: "A batch start closes and spends the record of a PR whose change is on main, and a dry run settles nothing"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestJourneySettleClosesAndSpendsALandedPRAtTheNextBatchStart$' ./cmd/evolve"
  - criterion: "A batch start spends the record of a host-closed PR with a printed line and keeps the record of an open PR that is not on main"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestJourneySettleSpendsTheRecordOfAClosedPRWithAPrintedLine$' ./cmd/evolve"
  - criterion: "A batch start closes an open batch PR whose members are all landed or closed and keeps one that has an open member"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestJourneySettleClosesAnOpenBatchPRWhoseMembersAreLandedOrClosed$' ./cmd/evolve"
---

# Eval: The train settles landed PRs and spends their records after an interrupted batch

> Pins the settle at the start of `evolve integration batch`. On 2026-11-17 the console found nine open PRs whose change was on main, and nine leftover batch PRs. Their records stayed in `.evolve/waves/integration-console/`, because `Train.settle` runs only at the end of the batch run that landed them. Source incident: the interrupted batch of 2026-11-16 (inbox item `integration-train-settles-landed-prs-after-an-interrupted-batch`, cycle 2456).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| landed-record | A landed PR is closed with the landed note and its record is spent at batch start | 4/10 | `go test -run TestJourneySettleClosesAndSpendsALandedPRAtTheNextBatchStart ./cmd/evolve` |
| closed-record | A host-closed PR has its record spent with one printed line; an open PR keeps its record | 5/10 | `go test -run TestJourneySettleSpendsTheRecordOfAClosedPRWithAPrintedLine ./cmd/evolve` |
| batch-pr | An open batch PR with only landed or closed members closes; one with an open member stays | 6/10 | `go test -run TestJourneySettleClosesAnOpenBatchPRWhoseMembersAreLandedOrClosed ./cmd/evolve` |
