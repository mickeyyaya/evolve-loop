---
score_cap:
  - criterion: "A subject at four blocks returns ActionRedesign from Decide, and at three blocks the decision is unchanged"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2621_001|TestC2621_002' ./acs/cycle2621"
  - criterion: "A second count at four blocks after one redesign returns ActionToOperator, and at three blocks the decision is unchanged"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC2621_003 ./acs/cycle2621"
  - criterion: "The host block count in state.json survives a resume and reaches the decision of evolve convergence decide"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2621_004 ./acs/cycle2621"
  - criterion: "A seat cannot write the count: its input is refused and the role guard denies the host record"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2621_005 ./acs/cycle2621"
  - criterion: "The core.State byte-stability allowlist names the additive blockCounts key"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2621_006 ./acs/cycle2621"
---

# Eval: Convergence redesign rung with one host block count

> Pins component R1 and R2 of `docs/plans/pipeline-blockers-2026-11.md`. The operator rule says a subject blocked more than three times goes to redesign, never to round four. Before cycle 2621, `convergence.Decide` had no redesign action, and no host record held one block count for each subject. The source is the inbox item `pipeline-blockers-r1-one-block-count`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| redesign-rung | Four blocks give ActionRedesign, three do not | 3/10 | `go test -tags acs -run 'TestC2621_001|TestC2621_002' ./acs/cycle2621` |
| operator-rung | A second count at four gives ActionToOperator | 3/10 | `go test -tags acs -run TestC2621_003 ./acs/cycle2621` |
| resume-journey | The count survives a resume through the real storage and the CLI | 4/10 | `go test -tags acs -run TestC2621_004 ./acs/cycle2621` |
| seat-boundary | A seat cannot write the count | 4/10 | `go test -tags acs -run TestC2621_005 ./acs/cycle2621` |
| state-key-allowlist | The additive state key blockCounts is in additiveStateKeys (audit round 3, H1) | 5/10 | `go test -tags acs -run TestC2621_006 ./acs/cycle2621` |
