---
score_cap:
  - criterion: "The orchestrator persona assigns no ship, release, dispatch or state-writing script to itself"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2562_001 ./acs/cycle2562"
  - criterion: "The orchestrator persona never tells itself to record failure state"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2562_002 ./acs/cycle2562"
  - criterion: "Every decision section and every verdict row states the proposal and the kernel decision"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2562_003 ./acs/cycle2562"
  - criterion: "The scanner flags the original imperative verdict table and spares kernel-attributed, negated and read-only lines"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2562_004 ./acs/cycle2562"
---

# Eval: Orchestrator persona decision sections propose only

> Pins `agents/evolve-orchestrator.md` as a proposer. The Go kernel decides, ships, dispatches and writes cycle state. The persona names no script that ships, releases, dispatches or writes cycle state as its own action. Source: cycle 2200 changed three sentences, and the section "Verdict Decision Tree (after Audit)" still told the persona to run `ship.sh`, `release-pipeline.sh` and `record-failure-to-state.sh`. Cycle 2551 rewrote the table but left script mentions and a record instruction in the memo and path sections; its predicate package was archived with the unshipped cycle, so cycle 2562 re-pins the contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-action-script | No ship, release, dispatch or state script assigned to the persona | 6/10 | `go test -tags acs -run TestC2562_001 ./acs/cycle2562` |
| no-self-record | The persona never records failure state itself | 6/10 | `go test -tags acs -run TestC2562_002 ./acs/cycle2562` |
| kernel-framing | Each decision section and verdict row names the proposal and the kernel decision | 7/10 | `go test -tags acs -run TestC2562_003 ./acs/cycle2562` |
| scanner-fixtures | The scanner flags the original table and spares benign lines | 5/10 | `go test -tags acs -run TestC2562_004 ./acs/cycle2562` |
