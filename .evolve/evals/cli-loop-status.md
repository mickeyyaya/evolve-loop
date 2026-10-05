---
score_cap:
  - criterion: "evolve loop status prints evolve status's loop line plus the lease heartbeat and changes nothing under the project"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_020' ./acs/cycle1791"
  - criterion: "evolve loop status --json emits one object whose loop key carries running, brake_engaged, cycle_id and phase, and no remote prs or ci data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_021' ./acs/cycle1791"
  - criterion: "evolve loop status exits 2 when .evolve is not a directory and 10 on a stray argument"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_022' ./acs/cycle1791"
  - criterion: "a positional goal that is exactly a reserved word (stop, status, help, plan, watch; any case, surrounding space) exits 10 naming the intended verb, evolve loop stop included"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_023' ./acs/cycle1791"
  - criterion: "a multi-word positional goal and an explicit --goal-text status still resolve as goals"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_024' ./acs/cycle1791"
  - criterion: "runtime-reference.md's Operator commands section documents evolve loop status"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_025' ./acs/cycle1791"
---

# Eval: evolve loop status and reserved-word goals

> Pins `evolve loop status [--json] [--project-root P]`, cycle 1791: a read-only report of the loop section of `evolve status` (running, brake, cycle, phase, lease heartbeat) that starts nothing. It also pins the guard that refuses a positional goal that is exactly a reserved word (status, stop, help, plan, watch) with exit 10, naming the intended verb. Before this change `evolve loop status`, the natural question, launched a batch whose goal was "status" (inbox item cli-loop-status, 2026-09-30 CLI inventory; reproduced in this cycle's RED run as `stop_reason: unfinished_cycle`). A multi-word goal must still launch as before.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| read-only-text | loop line + lease heartbeat, tree unchanged | 6/10 | `go test -tags acs -run TestC1791_020 ./acs/cycle1791` |
| json-shape | `{"loop": …}` with running/brake/cycle/phase | 6/10 | `go test -tags acs -run TestC1791_021 ./acs/cycle1791` |
| refusals | exit 2 unreadable snapshot, exit 10 stray argument | 7/10 | `go test -tags acs -run TestC1791_022 ./acs/cycle1791` |
| reserved-words | exit 10 naming the intended verb | 7/10 | `go test -tags acs -run TestC1791_023 ./acs/cycle1791` |
| goal-regression | multi-word and explicit goals unchanged | 8/10 | `go test -tags acs -run TestC1791_024 ./acs/cycle1791` |
| operator-doc | Operator commands section documents it | 5/10 | `go test -tags acs -run TestC1791_025 ./acs/cycle1791` |
