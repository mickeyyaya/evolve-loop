---
score_cap:
  - criterion: "each of the 13 rules has a red fixture with one finding and a green fixture with none"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2489_002' ./acs/cycle2489"
  - criterion: "the ratchet fails on a finding missing from the baseline and on a stale baseline row"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2489_007|TestC2489_008' ./acs/cycle2489"
  - criterion: "a test name inside a raw string is not a finding"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2489_003' ./acs/cycle2489"
  - criterion: "the new packages graduate into the apicover gate"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2489_011' ./acs/cycle2489"
---

# Eval: evolve test audit analyzer (T1)

> Pins the contract of `internal/testaudit` and `internal/fakecontract` from plan
> `docs/plans/test-quality-audit-2026-11.md` section 6 and D2. Source: cycle 2489,
> inbox item test-audit-t1-analyzer.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rules | red and green fixture per rule | 4/10 | `go test -tags acs -run TestC2489_002 ./acs/cycle2489` |
| ratchet | new finding and stale row both fail | 5/10 | `go test -tags acs -run 'TestC2489_007|TestC2489_008' ./acs/cycle2489` |
| ast-not-text | raw string is not a finding | 6/10 | `go test -tags acs -run TestC2489_003 ./acs/cycle2489` |
| graduation | apicover enforce lines and named tests | 6/10 | `go test -tags acs -run TestC2489_011 ./acs/cycle2489` |
