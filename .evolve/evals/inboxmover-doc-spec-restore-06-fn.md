---
score_cap:
  - criterion: "Every 06-F1..06-F11 follow-up bullet restores its file:line anchors and symbols"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1923_003' ./acs/cycle1923"
  - criterion: "The 06-F2 retirement record survives and no present-tense claim about the retired writer returns"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1923_004' ./acs/cycle1923"
  - criterion: "The restored follow-up block stays within the STE sentence limit and ste-lint reports nothing inside it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1923_005|TestC1923_006' ./acs/cycle1923"
---

# Eval: Restore the 06-Fn follow-up specifications in 06-inboxmover.md

> Pins carryover d5447aa660771fdaeba5e61094bdf1ea5. Cycle 1922 shortened the follow-up list to
> one-line bullets and deleted the targets and file:line anchors (audit finding M1). The restore
> returns them as short STE sentences under each bullet and keeps the 06-F2 retirement record.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| anchors-restored | each 06-Fn block holds its anchors | 6/10 | `go test -tags acs -run TestC1923_003 ./acs/cycle1923` |
| retirement-kept | 06-F2 record kept, no present tense | 7/10 | `go test -tags acs -run TestC1923_004 ./acs/cycle1923` |
| ste-clean | block sentences <= 25 words, lint clean | 6/10 | `go test -tags acs -run 'TestC1923_005|TestC1923_006' ./acs/cycle1923` |
