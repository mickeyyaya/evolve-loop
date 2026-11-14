---
score_cap:
  - criterion: "A tmuxtest.Main process that exits, is killed or panics leaves no socket or server of its own"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_001' ./acs/cycle2376"
  - criterion: "A tmuxtest.Main process that returns leaves no socket or server of its own, its own-suffixed server included"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_002' ./acs/cycle2376"
  - criterion: "evolve gc lists the servers of a dead test process and never those of a live one"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_003' ./acs/cycle2376"
  - criterion: "The gc reaper stops the servers of a dead test process and spares those of a live one"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_004' ./acs/cycle2376"
  - criterion: "Each changed function has 100% coverage and no changed Go file gains a comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_005|TestC2376_006' ./acs/cycle2376"
  - criterion: "The bridge and gc package notes change and pass the STE lint"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2376_007' ./acs/cycle2376"
---

# Eval: A test process's tmux server stops when the test process ends

> Pins the end of the `evolve-bridge-t<pid>` server family. A probe on 2026-11-14 showed that a test binary which exits, is killed or panics leaves a stale socket file for the main server and for each `-own` server. A binary that returns leaves the `-own` socket. `evolve gc` ignores `evolve-bridge-t<pid>-own` names, because its owner pattern ends at the pid.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| abnormal-end | no socket or server after exit, kill or panic | 4/10 | `TestC2376_001` |
| normal-end | no `-own` socket after a normal return | 5/10 | `TestC2376_002` |
| gc-list | dead owner listed, live owner spared | 5/10 | `TestC2376_003` |
| gc-stop | dead owner stopped, live owner spared | 5/10 | `TestC2376_004` |
| hygiene | 100% coverage, zero comments | 6/10 | `TestC2376_005`, `TestC2376_006` |
| docs | bridge and gc notes in STE | 6/10 | `TestC2376_007` |
