---
score_cap:
  - criterion: "A server whose owner pid is dead and whose socket sits in a registered test TMUX_TMPDIR is stopped by the orphan socket reaper"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2625_001 ./acs/cycle2625"
  - criterion: "A server whose owner pid is alive stays, and the reaper counts it as skipped live"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2625_002 ./acs/cycle2625"
  - criterion: "The reap removes only dead-owner sockets and signals no process"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2625_003 ./acs/cycle2625"
  - criterion: "The reaper sweeps only the dirs that a test binary registered"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2625_004 ./acs/cycle2625"
---

# Eval: The orphan socket reaper sees the test tmux socket dirs

> Pins the fix for plan PR 1585, check K10, component X8. On 2026-11-20, 282 tmux servers on sockets `evolve-bridge-t<N>` had ppid 1. `swarm.ReapOrphanSockets` had the owner-pid rule, but its lister read only the current `TMUX_TMPDIR`, and test binaries use `/tmp/evtmux*`. Each predicate registers a test dir through `tmuxtest.Main`, the production caller, and reaps through `swarm.ExecReapOrphanSockets`, the production entry of `evolve gc` and the loop.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dead-owner-stopped | Dead-owner server on a registered test dir is stopped | 4/10 | `go test -tags acs -run TestC2625_001 ./acs/cycle2625` |
| live-owner-stays | Live-owner server stays and counts as skipped live | 5/10 | `go test -tags acs -run TestC2625_002 ./acs/cycle2625` |
| no-collateral | Only dead-owner sockets go; no process is signalled | 5/10 | `go test -tags acs -run TestC2625_003 ./acs/cycle2625` |
| registered-only | Unregistered dirs are not swept | 6/10 | `go test -tags acs -run TestC2625_004 ./acs/cycle2625` |
