---
score_cap:
  - criterion: "record-pr writes no record when refs/pull/N/head is behind the head branch"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestRecordPRStalePullRefWritesNoRecordOfTheOldCommitList$' ./internal/integration"
  - criterion: "a pull ref that catches up inside the bound is recorded with the full commit list, and a current pull ref is not refused"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestRecordPRStalePullRefThatCatchesUpInsideTheBoundRecordsTheFullCommitList|TestRecordPRStalePullRefDoesNotRefuseAPullRefThatIsCurrent)$' ./internal/integration"
  - criterion: "a pull ref that never catches up is refused and the refusal names the pull-ref SHA and the head-branch SHA"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRecordPRStalePullRefThatNeverCatchesUpNamesBothSHAs$' ./internal/integration"
  - criterion: "the record-pr verb exits 1 for a stale pull ref and exits 0 for a current one"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestRecordPRStalePullRefExitsRefusedNamingBothSHAs|TestRecordPRStalePullRefLeavesACurrentPullRefAlone)$' ./cmd/evolve"
---

# Eval: record-pr refuses a pull ref behind the pushed branch

> GitHub moves refs/pull/N/head some seconds after a push. record-pr fetched that ref at once and listed the commits of a stale tip, so the batch later refused the PR. This eval pins the refusal, the bounded wait and the exit code. Source incident: cycle 2600 inbox item record-pr-refuses-a-pull-ref-behind-the-pushed-branch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-stale-record | a stale pull ref leaves no record | 3/10 | `TestRecordPRStalePullRefWritesNoRecordOfTheOldCommitList` |
| bounded-catch-up | a late pull ref is recorded in full | 4/10 | `TestRecordPRStalePullRefThatCatchesUpInsideTheBoundRecordsTheFullCommitList` |
| names-both-shas | the refusal names both SHAs | 4/10 | `TestRecordPRStalePullRefThatNeverCatchesUpNamesBothSHAs` |
| verb-exit-code | the verb exits 1 on a stale ref | 5/10 | `TestRecordPRStalePullRefExitsRefusedNamingBothSHAs` |
