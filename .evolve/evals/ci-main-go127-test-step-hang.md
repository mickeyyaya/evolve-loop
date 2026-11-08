---
score_cap:
  - criterion: "A guard test fails when a package starts real tmux in its tests without a TestMain that calls tmuxtest.Main"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestEveryTmuxTestPackageUsesTheOrphanGuard$' ./internal/tmuxtest"
  - criterion: "The internal/swarm test binary marks every inherited descriptor close-on-exec, so a tmux server it starts cannot hold the go test output pipe"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2216_002_' ./acs/cycle2216"
  - criterion: "The cmd/evolve test binary marks every inherited descriptor close-on-exec"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2216_003_' ./acs/cycle2216"
---

# Eval: no test binary leaves a tmux server holding the go test output pipe

> Pins the fix for main run 40265789886 (Go 1.27, ubuntu), whose test step hung for more than 4 h after the last package line. The runner killed an orphan `tmux: server` (18408) at cleanup. Static evidence names `cmd/evolve` and `internal/swarm` as the two packages that start real tmux without `tmuxtest.Main`. Source incident: cycle 2216.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard-test | A package that starts tmux without `tmuxtest.Main` fails the guard | 5/10 | `go test -run TestEveryTmuxTestPackageUsesTheOrphanGuard ./internal/tmuxtest` |
| swarm-fd | swarm test binary closes inherited descriptors on exec | 6/10 | `go test -tags acs -run TestC2216_002_ ./acs/cycle2216` |
| evolve-fd | cmd/evolve test binary closes inherited descriptors on exec | 6/10 | `go test -tags acs -run TestC2216_003_ ./acs/cycle2216` |
