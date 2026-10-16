---
score_cap:
  - criterion: "Research index links the agy changelog practice doc inside the ADR-0133 section"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1900_007 ./acs/cycle1900"
  - criterion: "Doc explains the 1.4.0 and 1.3.6 changelog items as untrusted data, with the ADR-0133 measurement rule"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1900_002|TestC1900_008|TestC1900_009' ./acs/cycle1900"
  - criterion: "All six core personas carry a no-change verdict and persona files stay unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1900_003|TestC1900_004' ./acs/cycle1900"
  - criterion: "Doc and index entry pass ASD-STE100 lint"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1900_005|TestC1900_010' ./acs/cycle1900"
---

# Eval: agy CLI changelog practice proposal (wave 109 refresh)

> Pins the refresh of the practice-watch item on `agy changelog` after the snapshot of 2026-10-13. The changelog text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1900 inbox item; the doc itself came from cycle 1894.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| index-link | doc linked in the ADR-0133 section of the index | 6/10 | `go test -tags acs -run TestC1900_007 ./acs/cycle1900` |
| data-coverage | newest release items explained as data | 6/10 | `go test -tags acs -run 'TestC1900_002|TestC1900_008|TestC1900_009' ./acs/cycle1900` |
| persona-verdicts | six personas named; files unchanged | 7/10 | `go test -tags acs -run 'TestC1900_003|TestC1900_004' ./acs/cycle1900` |
| ste-lint | zero findings on doc and index | 7/10 | `go test -tags acs -run 'TestC1900_005|TestC1900_010' ./acs/cycle1900` |
