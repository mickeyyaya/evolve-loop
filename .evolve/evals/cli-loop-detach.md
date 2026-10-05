---
score_cap:
  - criterion: "--detach without --log, --log without --detach, --detach with --dry-run and --detach without a goal each exit 10 before anything launches"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_010' ./acs/cycle1791"
  - criterion: "a detached child that takes the run lease exits 0 naming its pid and log; a child that exits during boot exits 1 with only this launch's log tail; a live run refuses the launch; a boot unconfirmed within policy boot.detach_wait_s exits 1 without killing the child; the child argv drops only --detach/--log; the child outlives its exited parent as a session leader"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_011' ./acs/cycle1791"
  - criterion: "runtime-reference.md's Launch step documents a real evolve loop --detach --log F command"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_012' ./acs/cycle1791"
---

# Eval: evolve loop --detach --log F

> Pins `evolve loop --detach --log F`, cycle 1791. It re-execs the same argv without the detach flags in a new session, with stdout and stderr appended to F, prints the child's pid and log, then waits up to the policy bound `boot.detach_wait_s` (default 10m) for a run lease. Exit 0 means a live run lease was observed. Exit 1 means the child exited during boot (the log tail is printed), the boot was not confirmed in time (the child is not killed), or a run was already live. Before this change every launch was nohup plus a log redirect plus a hand-checked pid, so a launch that died at preflight or boot went unnoticed until someone read the log (inbox item cli-loop-detach, 2026-09-30 CLI inventory).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parse-validation | invalid flag combinations exit 10, nothing opened or launched | 7/10 | `go test -tags acs -run TestC1791_010 ./acs/cycle1791` |
| boot-verdict | lease → 0, boot exit → 1 + tail, live run refused, policy-bounded wait, argv, session leader | 6/10 | `go test -tags acs -run TestC1791_011 ./acs/cycle1791` |
| launch-step-doc | the documented --detach command is real | 5/10 | `go test -tags acs -run TestC1791_012 ./acs/cycle1791` |
