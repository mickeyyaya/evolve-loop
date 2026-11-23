---
score_cap:
  - criterion: "gc reaps every suffixed test-socket server of a dead owner and spares live ones"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestReapOrphanSockets_ReapsSuffixedTestSocketsOfADeadOwner$' ./internal/swarm"
  - criterion: "gc --dry-run lists, and gc stops, each dead-owner test server and never a live one"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestGCSockets_ADryRunNamesEveryDeadOwnerTestServerAndNeverALiveOne|TestGCSockets_AnApplyStopsEveryDeadOwnerTestServerAndSparesALiveOne)$' ./cmd/evolve"
  - criterion: "the next test binary start stops the real tmux servers of dead test binaries"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2616_001_' ./acs/cycle2616"
---

# Eval: a test's private tmux server dies with its test process

> Pins the leak where a killed or timed-out test binary left its `evolve-bridge-t<pid>` tmux server running with `exit-empty off`. The gc owner-pid pattern rejected the suffixed sockets that `ownTmuxServer` creates, and `tmuxtest.Main` had no start-time reap. Source incident: cycle 2616, inbox item test-tmux-servers-die-with-their-test-process.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| gc-pattern | suffixed dead-owner sockets reaped, live spared | 4/10 | `go test -run TestReapOrphanSockets_ReapsSuffixedTestSocketsOfADeadOwner ./internal/swarm` |
| gc-cli | dry-run lists, apply stops, live never touched | 5/10 | `go test -run TestGCSockets_ ./cmd/evolve` |
| start-reap | next test binary start stops dead-owner servers | 5/10 | `go test -tags acs -run TestC2616_001_ ./acs/cycle2616` |
