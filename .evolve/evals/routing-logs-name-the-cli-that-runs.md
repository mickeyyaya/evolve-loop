---
score_cap:
  - criterion: "A deep seat whose chain starts with a CLI the tier ceiling forbids logs the dispatched CLI and the skipped candidate with the ceiling reason"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestLogDecision_ADeepSeatWhoseChainStartsWithAForbiddenCLILogsTheDispatchedCLI$' ./internal/phases/runner/"
  - criterion: "A bench ending on a later day prints its date in the bench reorder line; a bench ending today keeps the clock form"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestApplyCLIHealthBench_TheReorderLineCarriesTheDateOfABenchEndingOnALaterDay$' ./internal/bridgechain/"
  - criterion: "The bridge's launch session matches the runner's logged CLI in an end-to-end fake dispatch"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRun_TheBridgeLaunchesTheCLITheRunnerLogged$' ./internal/phases/runner/"
---

# Eval: The runner's dispatch line names the CLI that runs, and bench times carry their date

> Pins two operator-facing routing log lines. On wave 77 (cycles 1829/1830) both
> builders logged `cli=agy-tmux` while the deep tier ceiling ran them on
> `claude-tmux`, and the bench line read `agy-claude until 21:42` at 22:43 for a
> bench that ended the next day. `logDecision` must name the CLI that
> `cliroute.Decision.Walk()` selects, and `benchedSummary` must print the date of
> a bench that does not end today. Source incident: inbox item
> `routing-logs-name-the-cli-that-runs`, fixed in cycle 1926.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dispatched-cli-logged | the deep seat logs claude-tmux and the skipped agy-tmux with the deep ceiling | 8/10 | `go test -run TestLogDecision_ADeepSeatWhoseChainStartsWithAForbiddenCLILogsTheDispatchedCLI ./internal/phases/runner/` |
| bench-date | a later-day bench prints `2006-01-02 15:04`; a today bench keeps `15:04` | 6/10 | `go test -run TestApplyCLIHealthBench_TheReorderLineCarriesTheDateOfABenchEndingOnALaterDay ./internal/bridgechain/` |
| log-matches-launch | the runner's logged CLI equals the bridge's launched CLI | 8/10 | `go test -run TestRun_TheBridgeLaunchesTheCLITheRunnerLogged ./internal/phases/runner/` |
