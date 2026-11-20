---
score_cap:
  - criterion: "OQ2 churn of one rewritten line in ten added lines within 14 days is 10 percent, and a rewrite after 14 days does not count"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2569_001|TestC2569_002|TestC2569_003|TestC2569_004' ./acs/cycle2569"
  - criterion: "OQ3 counts a Revert subject and a This reverts commit body, ignores lookalikes and old commits, and gives 0 with the Wilson upper bound for a log with no revert"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2569_005|TestC2569_006|TestC2569_007|TestC2569_008' ./acs/cycle2569"
  - criterion: "OQ4 reports one recovery and the exact hours for one failed and one later shipped dossier, from the first failure, with the retrospective detector still at two failures"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2569_009|TestC2569_010|TestC2569_011|TestC2569_012|TestC2569_013' ./acs/cycle2569"
  - criterion: "The changed reader functions are at 100 percent coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2569_014|TestC2569_015' ./acs/cycle2569"
  - criterion: "OQ2 and OQ3 default to stage report and OQ4 to shadow, read through the evolve quality report command"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2569_016|TestC2569_017|TestC2569_018' ./acs/cycle2569"
---

# Eval: Quality indexes Q4, OQ2 churn, OQ3 revert rate and OQ4 recovery

> Pins the readers of the three outcome indexes of docs/architecture/llm-quality-indexes.md section 10. Source incident: the lane tries of cycles 2075, 2090 and 2107 failed on the OQ2 acceptance test. The OQ4 reader lives in `pipelinerates`, because `qualitypanel` cannot import `dossier` (the chain through `policy` makes an import cycle).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| churn-window | 10 percent churn, after-14-days rewrite not counted, empty window, non-repository | 4/10 | `go test -tags acs -run 'TestC2569_001..004'` |
| revert-parse | subject and body forms, lookalikes, window, Wilson bound for zero | 4/10 | `go test -tags acs -run 'TestC2569_005..008'` |
| recovery-hours | one failure then ship, first failure, median, no ship, no time | 4/10 | `go test -tags acs -run 'TestC2569_009..013'` |
| coverage | changed functions at 100 percent | 6/10 | `go test -tags acs -run 'TestC2569_014|015'` |
| stage-wiring | default stages through the command | 5/10 | `go test -tags acs -run 'TestC2569_016..018'` |
