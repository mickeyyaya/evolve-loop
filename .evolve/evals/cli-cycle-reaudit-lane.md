---
score_cap:
  - criterion: "evolve cycle reaudit rebases a moved-base cycle, audits it, and records a binding ship accepts"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./cmd/evolve -run 'TestCycleReaudit_MovedBaseRebasesAuditsAndShipBindingAccepts'"
  - criterion: "a rebase conflict or a live run lease exits 1 and leaves the worktree unchanged"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./cmd/evolve -run 'TestCycleReaudit_LiveLeaseExitsOneAndLeavesWorktreeUnchanged|TestCycleReaudit_RebaseConflictExitsOneAndLeavesWorktreeUnchanged'"
  - criterion: "the verb and the recovery chain call one shared rebase step"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core -run 'TestReauditCycle_AndRecoveryChainShareOneRebaseStep'"
---

# Eval: evolve cycle reaudit verb (lane part)

> Pins the `evolve cycle reaudit --cycle N` verb and the `core.ReauditCycle` function it shares with the ship recovery chain. Source incident: cycle 1979 inbox item cli-cycle-reaudit-lane. Before this verb the operator resumed a cycle and hoped, or hand-built an audit PhaseRequest.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| moved-base | rebase, audit, record a ship-bindable verdict | 4/10 | `go test ./cmd/evolve -run TestCycleReaudit_MovedBaseRebasesAuditsAndShipBindingAccepts` |
| refusal | live lease or conflict exits 1, worktree unchanged | 5/10 | `go test ./cmd/evolve -run 'the LiveLease and RebaseConflict tests'` |
| one function | verb and chain share one rebase step | 6/10 | `go test ./internal/core -run TestReauditCycle_AndRecoveryChainShareOneRebaseStep` |
