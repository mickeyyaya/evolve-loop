---
score_cap:
  - criterion: "bare evolve gc refuses before any reaper (tmux) runs"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_001 ./acs/cycle1801"
  - criterion: "gc help states the refusal instead of promising a cwd default for -project-root"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_002 ./acs/cycle1801"
  - criterion: "a non-mutating gc (--dry-run, with or without a root) still reaches the reapers"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_003 ./acs/cycle1801"
  - criterion: "no cmd/evolve test references the swarm.Exec* host reapers, which stay injectable through gcReapers"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_004 ./acs/cycle1801"
---

# Eval: gc validates before it reaps

> Pins validation-first ordering in `evolve gc` (cycles 1798, 1800 and 1801). A bare
> run reached the tmux reapers before the project-root refusal, and a cmd/evolve
> test hit the host tmux server. Cycle 1801 adopted cycle 1800's continuation and
> carries the pins forward. The predicates drive the real binary with a fake `tmux`
> on PATH. Against main's pre-fix cmd_gc.go, TestC1801_001 records
> `-L evolve-bridge list-sessions` before the refusal, and TestC1801_002 fails
> on the help text.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| refuse-first | zero tmux calls on bare gc | 4/10 | `go test -tags acs -run TestC1801_001 ./acs/cycle1801` |
| honest-help | help states refusal | 7/10 | `go test -tags acs -run TestC1801_002 ./acs/cycle1801` |
| anti-no-op | dry-run still reaps (previews) | 5/10 | `go test -tags acs -run TestC1801_003 ./acs/cycle1801` |
| host-isolation | no test references real reapers; seam exists | 5/10 | `go test -tags acs -run TestC1801_004 ./acs/cycle1801` |
