---
score_cap:
  - criterion: "an item that fails twice with one fingerprint carries a changed plan on its third run, and at the default ceiling it is never retried"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_001|TestC2254_002' ./acs/cycle2254"
  - criterion: "the second run carries the findings of the first run and a raised-effort plan, and a system-level failure stamps neither"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_003|TestC2254_004' ./acs/cycle2254"
  - criterion: "after the final run the ceiling policy routes the item to the console or quarantines it, and a lane cannot claim it again"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_005|TestC2254_006' ./acs/cycle2254"
  - criterion: "a quarantine release clears the stale plan and findings, and the round stamps belong to the loop"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_007|TestC2254_009' ./acs/cycle2254"
  - criterion: "a route failure at the ceiling never returns the item to the retry pool, and the carried findings stay valid text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_011|TestC2254_012' ./acs/cycle2254"
  - criterion: "the task contract of the next run carries the prior findings and plan, and treats them as data"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestComposeTaskContract_PriorRun' ./internal/core"
  - criterion: "PlanRound schedules the rounds and every function of the round lifecycle files has 100% coverage"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2254_008|TestC2254_010' ./acs/cycle2254"
---

# Eval: Convergence V11, each cycle run on an inbox item is a round

> Pins the inbox-item loop of the convergence policy (plan component V11, decision V-D12). The second run of an item carries the findings of the first run and a raised-effort plan. The third run changes the plan. After the final run the ceiling policy routes the item to the console or quarantines it. Source: the inbox item `convergence-policy-v11-inbox-item-rounds`, cycle 2240 (the first build) and cycle 2254 (the audit gaps M1, L1 and L3 of cycle 2240).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| replay-changes-plan | two same-fingerprint failures change the third plan; the default ceiling parks the item | 3/10 | `go test -tags acs -run 'TestC2254_001\|TestC2254_002' ./acs/cycle2254` |
| second-run-carry | findings and a raised-effort plan reach the second run; a system fault stamps nothing | 4/10 | `go test -tags acs -run 'TestC2254_003\|TestC2254_004' ./acs/cycle2254` |
| final-run-exit | the ceiling policy routes to the console or quarantines | 4/10 | `go test -tags acs -run 'TestC2254_005\|TestC2254_006' ./acs/cycle2254` |
| release-clears | a quarantine release clears the stamps; the stamps are loop-owned | 5/10 | `go test -tags acs -run 'TestC2254_007\|TestC2254_009' ./acs/cycle2254` |
| route-failure-parks | a failed console route at the ceiling falls through to quarantine; the cut findings are valid text | 5/10 | `go test -tags acs -run 'TestC2254_011\|TestC2254_012' ./acs/cycle2254` |
| contract-carries-prior-run | the task contract of a retried item carries the findings and the plan, as data | 5/10 | `go test -run '^TestComposeTaskContract_PriorRun' ./internal/core` |
| round-schedule-coverage | PlanRound schedules the rounds; the lifecycle functions have 100% coverage | 4/10 | `go test -tags acs -run 'TestC2254_008\|TestC2254_010' ./acs/cycle2254` |
