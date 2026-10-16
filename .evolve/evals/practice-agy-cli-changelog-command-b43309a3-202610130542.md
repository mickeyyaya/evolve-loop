---
score_cap:
  - criterion: "Research doc covers agy changelog versions 1.3.4 to 1.4.0 as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1894_002 ./acs/cycle1894"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 and states why none changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1894_003|TestC1894_004' ./acs/cycle1894"
  - criterion: "Doc passes ASD-STE100 lint"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1894_005 ./acs/cycle1894"
---

# Eval: agy CLI changelog practice proposal

> Pins the research proposal for the practice-watch item on `agy changelog`. The changelog text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1894 inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | versions 1.3.4 to 1.4.0 explained as data | 6/10 | `go test -tags acs -run TestC1894_002 ./acs/cycle1894` |
| persona-verdicts | six personas named with a reason; persona files unchanged | 7/10 | `go test -tags acs -run 'TestC1894_003|TestC1894_004' ./acs/cycle1894` |
| ste-lint | zero ste-lint findings | 7/10 | `go test -tags acs -run TestC1894_005 ./acs/cycle1894` |
