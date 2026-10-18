---
score_cap:
  - criterion: "V1-V3, V5-V9, V11, V13 and V14 are each resolved in the design, plan or dossier"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1945_00[1-9]|TestC1945_01[01]' ./acs/cycle1945"
  - criterion: "review-loop-and-quality-index.md links ADR-0126 where its loop calls the convergence policy"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1945_012' ./acs/cycle1945"
  - criterion: "the landing notes cite a resolving file:line for each resolved finding"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1945_013' ./acs/cycle1945"
  - criterion: "every relative link in the touched docs resolves"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1945_01[45]' ./acs/cycle1945"
---

# Eval: Apply the E0 verify check's filed findings

> Pins the doc fixes for the 11 findings filed by the ADR-0126 verify-only check of the ADR-0125 docs (cycle 1945, inbox `audit-contract-e0-review-followups`). Each predicate reads the doc section the finding names.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| finding-fixes | V1-V3, V5-V9, V11, V13, V14 | 6/10 | `go test -tags acs -run 'TestC1945_00|TestC1945_01[01]'` |
| adr-link | index links ADR-0126 | 7/10 | `go test -tags acs -run TestC1945_012` |
| landing-notes | file:line refs resolve | 6/10 | `go test -tags acs -run TestC1945_013` |
| links | relative links resolve | 7/10 | `go test -tags acs -run 'TestC1945_01[45]'` |
