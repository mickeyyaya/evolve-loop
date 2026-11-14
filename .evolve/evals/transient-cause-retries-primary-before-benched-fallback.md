---
score_cap:
  - criterion: "A submit_wedged primary with only benched fallbacks is retried once in a fresh session and the retry answer is returned"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2388_001' ./acs/cycle2388"
  - criterion: "The retry happens once only, then the walk continues to the benched family"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2388_002' ./acs/cycle2388"
  - criterion: "A non-transient primary failure walks the chain in bench order with no retry"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2388_003|TestC2388_005' ./acs/cycle2388"
  - criterion: "The package note records the rule in STE"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2388_006|TestC2388_007' ./acs/cycle2388"
---

# Eval: A transient launch cause retries the primary before the chain walks to a benched family

> Pins the `bridgechain.Walking.Launch` rule that a transient primary failure (exit 81, cause `submit_wedged`) with only benched fallbacks gets one fresh session of the primary before the walk moves on. Source incident: cycle 2090, where claude-tmux exited 81 `submit_wedged`, the chain walked to the benched agy-tmux, and the cycle failed with exit 85.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| retry-then-return | The fresh session answer is returned | 4/10 | `go test -tags acs -run TestC2388_001 ./acs/cycle2388` |
| retry-once | No second retry, then the walk continues | 6/10 | `go test -tags acs -run TestC2388_002 ./acs/cycle2388` |
| bench-order-preserved | A non-transient failure walks with no retry | 6/10 | `go test -tags acs -run 'TestC2388_003|TestC2388_005' ./acs/cycle2388` |
| doc-recorded | The note records the rule in STE | 7/10 | `go test -tags acs -run 'TestC2388_006|TestC2388_007' ./acs/cycle2388` |
