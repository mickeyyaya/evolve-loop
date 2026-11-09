---
score_cap:
  - criterion: "A SIGKILLed test binary that ran through tmuxtest.Main leaves no tmux server, even when a test cleanup removed the socket directory"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC2237_001 ./acs/cycle2237"
  - criterion: "A normal run through tmuxtest.Main leaves no tmux server, even when a test cleanup removed the socket directory"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2237_002 ./acs/cycle2237"
  - criterion: "The teardown kills no process the test binary did not start (lookalike socket, default socket, plain process)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2237_003 ./acs/cycle2237"
  - criterion: "The tmuxtest package tests stay green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/tmuxtest"
---

# Eval: tmuxtest servers outlive their test binary

> A tmux server that tmuxtest.Main starts keeps running when a test cleanup removes the socket directory, because the stop path and the orphan guard both reach the server through the socket. Cycle 2216 left 481 such servers; the host held 1,333 tmux processes on 2026-11-09. Each predicate runs a real helper binary through tmuxtest.Main in a private TMUX_TMPDIR and reads the server pid, so a no-op change cannot pass.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sigkill | no server after SIGKILL, socket directory intact or removed | 3/10 | `go test -tags acs -run TestC2237_001 ./acs/cycle2237` |
| normal-exit | no server after a normal run, socket directory intact or removed | 4/10 | `go test -tags acs -run TestC2237_002 ./acs/cycle2237` |
| no-collateral | bystander process and lookalike or default servers survive | 5/10 | `go test -tags acs -run TestC2237_003 ./acs/cycle2237` |
| package-green | existing package tests pass | 6/10 | `go test -count=1 ./internal/tmuxtest` |
