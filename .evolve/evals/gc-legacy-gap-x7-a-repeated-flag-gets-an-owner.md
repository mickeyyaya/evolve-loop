---
score_cap:
  - criterion: "A flag that repeats on two evolve gc runs files one inbox item, not two"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2295_001|TestC2295_003|TestC2295_004' ./acs/cycle2295"
  - criterion: "The gc report carries the repeat count of each flag, and a dry run neither counts nor files"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2295_002|TestC2295_005' ./acs/cycle2295"
  - criterion: "Each changed function of go/internal/gc has 100% coverage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2295_006' ./acs/cycle2295"
  - criterion: "The changed functions of cmd/evolve gc wiring have 100% coverage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2295_008' ./acs/cycle2295"
  - criterion: "The build explanation document names D11 and X7"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2295_007' ./acs/cycle2295"
---

# Eval: A repeated gc flag gets an owner

> Pins D11 and X7 of docs/plans/gc-closes-the-legacy-gap-2026-11.md. Each flag
> that repeats on two `evolve gc` runs goes into the report with a count, and
> once into the inbox. Source: the 157 flags of F8 that went to no owner.
> The predicates run the built `evolve gc` binary on a git hub fixture, so
> they also prove that the production caller reaches the tally (cycle 2295).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-item | two runs file one item; three or four runs add none | 4/10 | `TestC2295_001`, `TestC2295_003`, `TestC2295_004` |
| count-report | report line carries `count=N`; dry run does not count | 5/10 | `TestC2295_002`, `TestC2295_005` |
| coverage | changed gc functions at 100% | 5/10 | `TestC2295_006` |
| coverage-cmd | changed cmd/evolve gc functions at 100% | 5/10 | `TestC2295_008` |
| decision-ids | explanation names D11 and X7 | 6/10 | `TestC2295_007` |
