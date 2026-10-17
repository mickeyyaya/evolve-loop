---
score_cap:
  - criterion: "Every mention of RecordRootTaskFailure in 06-inboxmover.md records the 06-F2 retirement in cycle 1864"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1922_002' ./acs/cycle1922"
  - criterion: "Every mention of RecordRootTaskFailure in internal-inboxmover.md records the 06-F2 retirement in cycle 1864"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1922_003' ./acs/cycle1922"
  - criterion: "No doc describes the retired writer as present"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1922_004|TestC1922_005' ./acs/cycle1922"
---

# Eval: Record the 06-F2 retirement of RecordRootTaskFailure in the inboxmover docs

> Pins the doc rewrite of carryover d8f37e0a7321d83524f6f7bdfe7a2ef71. Cycle 1864 deleted
> `RecordRootTaskFailure`, but 06-inboxmover.md and internal-inboxmover.md still described it
> as present. Source incident: cycle 1922 scout/triage.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| decomposition-record | each mention names 06-F2, cycle 1864 and a retirement word | 7/10 | `go test -tags acs -run TestC1922_002` |
| package-record | same for the package doc | 7/10 | `go test -tags acs -run TestC1922_003` |
| no-present-tense | stale present-tense claims removed | 6/10 | `go test -tags acs -run 'TestC1922_004|TestC1922_005'` |
