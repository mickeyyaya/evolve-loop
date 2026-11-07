---
score_cap:
  - criterion: "The private tmux row finds its server when each tmux client connects late"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer$' ./internal/bridge"
  - criterion: "The private tmux row keeps a server and a session for its window when the session shell ends at once"
    max_if_missing: 7
    evidence: "cd go && go test -tags integration -count=1 -run '^TestPrivateTmuxRow_TheServerOutlivesTheSessionShellThatEndsBeforeTheWindowStarts$' ./internal/bridge"
  - criterion: "The private tmux row waits on a readiness probe and names a server that never became ready, with the cause of tmux"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^(TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep|TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady|TestPrivateTmuxRow_AServerThatNeverStartsIsReportedWithTheCauseOfTmux)$' ./internal/bridge"
  - criterion: "An unloaded host still gets a window on its own pty"
    max_if_missing: 5
    evidence: "cd go && go test -tags integration -count=1 -run '^TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty$' ./internal/bridge"
---

# Eval: TestAgentSandboxContract private tmux server flake

> Pins the contract row "start a private tmux server (-L) and run a window on its own pty". Under host load the window command ended and took the only session, and so the server, with it. The late `tmux wait-for ran` of the cell then failed with `no server running`. Source incident: inbox item bridge-sandbox-contract-private-tmux-flake (two floor failures on 2026-10-12, cycle 2191 audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| late-client | Row passes when every cell-side tmux client is delayed | 6/10 | `TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer` |
| session-shell | Row passes when the shell of the first session ends at once and the server would exit with it (cycle 2214) | 7/10 | `TestPrivateTmuxRow_TheServerOutlivesTheSessionShellThatEndsBeforeTheWindowStarts` |
| readiness | Row has no sleep and names a server that never became ready, with the words of tmux | 6/10 | `TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep`, `TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady`, `TestPrivateTmuxRow_AServerThatNeverStartsIsReportedWithTheCauseOfTmux` |
| preservation | Row still runs the window on a pty of its own | 5/10 | `TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty` |
