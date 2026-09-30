---
score_cap:
  - criterion: "A dead-pid canary socket in the host's tmux socket dir survives `go test ./cmd/evolve/` (the whole package), and the sweep-reaching loop tests still execute and pass"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestC1774_001_HostTmuxCanarySurvivesCmdEvolveSuite -tags acs ./acs/cycle1774/"
  - criterion: "The loop's orphan-socket sweep is redirected, not disabled: a batch still reaps a dead-pid per-run socket and spares a live-pid one in the socket dir TMUX_TMPDIR names"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunLoopBatch_OrphanSocketGCReapsOnlyDeadSocketsInTheEnvNamedDir$' ./cmd/evolve/"
---

# Eval: Loop-batch tests stop reaping the host's tmux sockets

> Pins inbox item `loop-batch-tests-reap-host-tmux-sockets` (2026-09-29,
> weight 0.45): `runLoopBatch`'s startup sweep and `reapCycleSessions`'
> post-cycle sweep both call `gcOrphanSessions` →
> `swarm.ExecReapOrphanSockets`, which lists and unlinks dead-pid
> `evolve-bridge-p<pid>` socket files under `$TMUX_TMPDIR/tmux-<uid>/`
> (default `/tmp`). Unit tests in `go/cmd/evolve` therefore mutated the host's
> real tmux socket dir. Source incident: cycle 1774 — bug-reproduction showed a
> canary in `/tmp/tmux-501/` deleted by the five inbox-named tests; the tdd
> phase then measured 65 distinct tests sweeping the host dir, 13 of them
> still sweeping it after isolating only `installStubDeps`, so a fix scoped
> to that one helper leaves the acceptance red.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| host-isolation | Canary in the stand-in host socket dir survives the whole `./cmd/evolve` suite; the 18 named sweep-reaching tests run and pass | 8/10 | `go test -run TestC1774_001_HostTmuxCanarySurvivesCmdEvolveSuite -tags acs ./acs/cycle1774/` |
| sweep-not-disabled | The loop's real sweep still reaps dead / spares live sockets in the TMUX_TMPDIR-named dir | 7/10 | `go test -run '^TestRunLoopBatch_OrphanSocketGCReapsOnlyDeadSocketsInTheEnvNamedDir$' ./cmd/evolve/` |
