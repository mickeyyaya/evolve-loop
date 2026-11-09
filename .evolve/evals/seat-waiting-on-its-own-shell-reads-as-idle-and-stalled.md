---
score_cap:
  - criterion: "A footer that shows a running shell is parsed into the pane snapshot, and only the footer counts"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRunningShells_ReadsTheCountFromTheFooter$' ./internal/bridge/panestream"
  - criterion: "The bridge wait loop publishes the shell count from the footer"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestRunTmuxREPL_PublishesTheRunningShellCountFromTheFooter|TestRunTmuxREPL_PublishesNoShellsWhenTheFooterShowsNone)$' ./internal/bridge"
  - criterion: "A pane with a running shell raises no LIVENESS_PHASE_STALLED before the policy shell limit, and states the wait on its shell at that limit"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2234_00[45]' ./acs/cycle2234"
  - criterion: "A pane with no running shell and no new transcript still raises the stall signal"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2234_006' ./acs/cycle2234"
---

# Eval: A seat that waits on its own shell is not stalled

> Pins the shell-aware stall rule of the pane observer. A seat that waits on a
> background shell shows no new transcript, so the plain stall limit raised
> LIVENESS_PHASE_STALLED on a healthy seat (wave 159, cycles 2205 and 2206).
> The footer shell count now selects a longer limit from policy.json
> (`observer.shell_stall_s`); a pane without a shell keeps the plain limit.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| footer-parse | footer count only, malformed counts read as 0 | 6/10 | `go test -run TestRunningShells_ReadsTheCountFromTheFooter ./internal/bridge/panestream` |
| bridge-publish | wait loop writes Shells to the snapshot | 6/10 | `go test -run TestRunTmuxREPL_Publishes ./internal/bridge` |
| shell-limit | no stall before the shell limit, reason names the shell | 7/10 | `go test -tags acs -run TestC2234_00[45] ./acs/cycle2234` |
| no-shell-stall | frozen pane without a shell still stalls | 7/10 | `go test -tags acs -run TestC2234_006 ./acs/cycle2234` |
