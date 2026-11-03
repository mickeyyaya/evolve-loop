---
score_cap:
  - criterion: "evolve cycle models --cycle N prints the tier, selector and serving model of each phase from llm-calls.ndjson"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2132_001 ./acs/cycle2132"
  - criterion: "a launch through a fake driver that reports a model records it on the llm-calls line"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2132_002 ./acs/cycle2132"
  - criterion: "a record without a serving model prints unreported and a cycle without records exits 1"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2132_003 ./acs/cycle2132"
  - criterion: "runtime-reference.md documents every flag and exit code of evolve cycle models"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2132_004|TestC2132_005' ./acs/cycle2132"
  - criterion: "the pr refresh integration test and the research index are not weakened"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2132_006|TestC2132_007' ./acs/cycle2132"
---

# Eval: evolve cycle models (console part)

> Pins the read-only verb `evolve cycle models --cycle N [--json]` and the served-model field of the llm-calls
> record. The command, the recording and the operator bullet exist at the base of cycle 2132. These pins stop a
> regression and the two scope violations that failed cycle 2122 (a weakened pr refresh test, a deleted index row).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reader-exists | Per-phase tier, selector and served model are printed | 4/10 | `go test -tags acs -run TestC2132_001 ./acs/cycle2132` |
| recording-held | Fake-driver launches record the served model | 5/10 | `go test -tags acs -run TestC2132_002 ./acs/cycle2132` |
| unreported-fallback | Missing served model prints unreported; bad input fails loudly | 6/10 | `go test -tags acs -run TestC2132_003 ./acs/cycle2132` |
| docs-bullet | Bullet names each real flag and exit code, passes strict STE lint | 5/10 | `go test -tags acs -run 'TestC2132_004|TestC2132_005' ./acs/cycle2132` |
| scope-fences | pr refresh test and research index unchanged (cycle 2122 audit H1, H2) | 4/10 | `go test -tags acs -run 'TestC2132_006|TestC2132_007' ./acs/cycle2132` |
