---
score_cap:
  - criterion: "contractHost.agent returns within 15 s when a daemon child of the agent holds stdout"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestAgentWaitDelay_ReturnsWhileADaemonChildHoldsStdout$' ./internal/bridge"
  - criterion: "no private tmux -L server of a contract row outlives the row"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestAgentWaitDelay_NoPrivateTmuxServerOutlivesTheRow$' ./internal/bridge"
  - criterion: "every top-level test of the bridge package is parallel or listed serial, new waitdelay tests included"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial$' ./internal/bridge"
---

# Eval: the sandbox contract agent call has a WaitDelay

> Pins the test-only fix of inbox item sandbox-contract-agent-waitdelay. CI macOS on PR #1193 hung
> in TestAgentSandboxContract/start_a_private_tmux_server_(-L) for the 20 minute package timeout,
> because contractHost.agent called CombinedOutput with no cmd.WaitDelay while a daemon child held the pipe.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| daemon-holds-stdout | agent returns within 15 s | 6/10 | `go test -tags integration -run '^TestAgentWaitDelay_ReturnsWhileADaemonChildHoldsStdout$' ./internal/bridge` |
| tmux-server-reaped | no tmux -L server survives the row | 6/10 | `go test -tags integration -run '^TestAgentWaitDelay_NoPrivateTmuxServerOutlivesTheRow$' ./internal/bridge` |
| package-parallel-guard | the package guard passes with the new tests | 7/10 | `go test -run '^TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial$' ./internal/bridge` |
