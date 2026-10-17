---
score_cap:
  - criterion: "Research doc explains the Releases and × lines and the truncated keyboard focus line as untrusted data, located in the snapshot"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1919_002|TestC1919_003|TestC1919_005' ./acs/cycle1919"
  - criterion: "Doc keeps older release items and gives every core persona a no-change verdict that weighs the keyboard fix; persona files stay unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1919_006|TestC1919_007|TestC1919_008|TestC1919_010' ./acs/cycle1919"
  - criterion: "Research index links the doc under ADR-0133 and both files pass ASD-STE100 lint"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1919_009|TestC1919_011' ./acs/cycle1919"
---

# Eval: agy CLI changelog page snapshot diff

> Pins the research proposal for the fifth change of the agy CLI changelog page. The diff excerpt holds the search bar labels `Releases` and `×` and a line cut at the 64 KiB snapshot limit, so the proposal must read them as data and invent no feature. Source: cycle 1919 inbox item practice-agy-cli-changelog-page-c71ffe53-202610170204.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| diff-as-data | diff lines explained and located; no invented claims | 6/10 | `go test -tags acs -run 'TestC1919_002|TestC1919_003' ./acs/cycle1919` |
| persona-verdicts | six personas with a no-change reason; persona files unchanged | 7/10 | `go test -tags acs -run 'TestC1919_007|TestC1919_008' ./acs/cycle1919` |
| index-and-lint | index link under ADR-0133; zero ste-lint findings | 7/10 | `go test -tags acs -run 'TestC1919_009|TestC1919_011' ./acs/cycle1919` |
