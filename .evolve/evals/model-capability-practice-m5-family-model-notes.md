---
score_cap:
  - criterion: "A dispatch resolves the notes of its own family only, through the Families selector of an overlay rule"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2245_001|TestC2245_002|TestC2245_003' ./acs/cycle2245"
  - criterion: "A family with no notes file gets no overlay and no error"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2245_004|TestC2245_009' ./acs/cycle2245"
  - criterion: "The practice watch item names the notes file of its family"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2245_008' ./acs/cycle2245"
  - criterion: "No family notes ride the default preload, so no unprotected skill reaches a prompt without a policy rule"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2245_005' ./acs/cycle2245"
---

# Eval: Model notes for each CLI family (practice M5)

> Pins the contract of practice M5 of ADR-0133: one notes file for each CLI family, and an overlay rule that selects the notes by the family of the dispatch. The source incident is cycle 2116, whose audit failed on H1 because the lane preloaded the unprotected notes into every prompt by default. The notes now load only through an explicit `families` rule, and the compiled default does not carry them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| family-selection | A claude dispatch gets the claude notes and not the agy notes | 6/10 | `go test -tags acs -run TestC2245_001 ./acs/cycle2245` |
| unknown-family | A family with no notes gets no overlay and no error | 7/10 | `go test -tags acs -run TestC2245_004 ./acs/cycle2245` |
| watch-text | The watch item names the notes file of its family | 6/10 | `go test -tags acs -run TestC2245_008 ./acs/cycle2245` |
| no-default-preload | The compiled default carries no family notes | 8/10 | `go test -tags acs -run TestC2245_005 ./acs/cycle2245` |
