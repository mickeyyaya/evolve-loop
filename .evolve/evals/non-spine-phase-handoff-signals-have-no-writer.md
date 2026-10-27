---
score_cap:
  - criterion: "A test-amplification run that adds tests yields amplify.tests_added in the routing digest"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2050_001' ./acs/cycle2050"
  - criterion: "mutation-gate is admissible after such a run in a routing replay"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2050_002' ./acs/cycle2050"
  - criterion: "mutation-gate stays inadmissible when the run added no tests"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2050_003' ./acs/cycle2050"
  - criterion: "The handoff writer drops a signal the phase spec does not declare"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2050_004' ./acs/cycle2050"
  - criterion: "The handoff writer serves every non-spine phase that declares outputs.signals"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2050_005' ./acs/cycle2050"
---

# Eval: Non-spine phase handoff signals have a writer

> The router reads `handoff-<phase>.json` for a completed non-spine phase, and the phase catalog counts `outputs.signals` as a producer. No runtime code wrote that file, so `amplify.tests_added` never reached the digest and mutation-gate stayed unreachable. Source incident: the review of PR #1105 (inbox item `non-spine-phase-handoff-signals-have-no-writer`), reproduced in cycle 2050.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| digest-has-signal | amplify.tests_added reaches the digest | 3/10 | `go test -tags acs -run '^TestC2050_001' ./acs/cycle2050` |
| gate-admissible | mutation-gate admissible after tests were added | 3/10 | `go test -tags acs -run '^TestC2050_002' ./acs/cycle2050` |
| gate-closed | mutation-gate inadmissible with zero tests added | 5/10 | `go test -tags acs -run '^TestC2050_003' ./acs/cycle2050` |
| declared-only | undeclared signal keys are dropped | 5/10 | `go test -tags acs -run '^TestC2050_004' ./acs/cycle2050` |
| generic-writer | writer covers other non-spine phases | 5/10 | `go test -tags acs -run '^TestC2050_005' ./acs/cycle2050` |
