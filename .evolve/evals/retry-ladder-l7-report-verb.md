---
score_cap:
  - criterion: "BuildReport counts tries, fixes and tokens per rung and class exactly on fixture run dirs"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLadderReport_ExactCountsPerRungAndClass|TestLadderReport_FixRequiresARetryThatPassed|TestLadderReport_RunsAreIndependent' ./internal/retryladder"
  - criterion: "evolve ladder report prints the exact per-rung and per-class counts as text and as JSON"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLadderReport_VerbPrintsExactCountsPerRungAndClass|TestLadderReport_VerbJSONCarriesTheSameCounts' ./cmd/evolve"
  - criterion: "the ladder report verb writes nothing, on success and on every failure path"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLadderReport_VerbWritesNothing' ./cmd/evolve"
  - criterion: "an absent runs dir and bad usage fail loudly with exit 2 and 10"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestLadderReport_VerbExitCodes|TestLadderReport_LoadRunsFailsLoudlyOnAnAbsentRunsDir' ./cmd/evolve ./internal/retryladder"
---

# Eval: Retry ladder L7, evolve ladder report

> Pins the read-only report of ADR-0142 component L7 (spec section 6 of
> docs/architecture/retry-ladder.md). The verb reads llm-calls.ndjson of each
> run dir and prints, per rung and failure class, the tries, the fixes (a retry
> whose exit code is 0) and the tokens. Source: inbox item
> retry-ladder-l7-report-verb, cycle 2014.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exact-counts | Aggregator counts match the fixture | 5/10 | `go test -run TestLadderReport_ExactCountsPerRungAndClass` |
| verb-output | Text and JSON carry the exact counts | 5/10 | `go test -run TestLadderReport_VerbPrintsExactCountsPerRungAndClass` |
| read-only | The verb changes no file | 4/10 | `go test -run TestLadderReport_VerbWritesNothing` |
| fail-loud | Absent runs dir and bad flags exit non-zero | 7/10 | `go test -run TestLadderReport_VerbExitCodes` |
