# Build Explanation — Cycle 1926

## Build Binding
- Cycle: 1926
- Base SHA: f6aeee9be9ffc625709682d19bd2fca82c4993bb

## Summary
The runner's dispatch line now names the CLI that the tier ceiling lets run, and it logs each candidate that the ceiling skips. The bench reorder line now prints the date of a bench that does not end today.

## Rationale
On wave 77 a deep builder logged `cli=agy-tmux` while the deep ceiling ran it on `claude-tmux`. `logDecision` printed `d.Plan.Candidates[0]`, the raw chain head. `cliroute.Decision.Walk()` already gives the permitted chain, so the log reuses it. The bench line read `until 21:42` for a bench that ended the next day, because `benchedSummary` used the clock form only.

## Changed Areas
- `go/internal/phases/runner/routing.go` — `logDecision` logs the plan of the new `dispatchedPlan`. That function walks the decision with `Decision.Walk()`, logs a `skipped <cli>: tier <tier> ceiling permits <families>` line for each forbidden candidate, and keeps the plan's primary source. A refused walk logs the refusal and falls back to the raw plan.
- `go/internal/bridgechain/bridgechain.go` — `benchedSummary` takes the clock time. The new `benchEnd` prints `15:04` for a bench that ends today and `2006-01-02 15:04` for a later day. `ApplyCLIHealthBench` defaults a nil clock to `time.Now`.
- `go/internal/phases/runner/routing_dispatched_cli_test.go` — the TDD tests for the logged CLI, the skip line and the log-matches-launch check.
- `go/internal/bridgechain/bench_summary_date_test.go` — the TDD tests for the bench date format.
- `go/acs/cycle1926/predicates_test.go` — the cycle predicates that run those tests and the existing routing log tests.
- `.evolve/evals/routing-logs-name-the-cli-that-runs.md` — the eval for this task.

## Design Decisions
The log reuses `Decision.Walk()`, the walk that `Router.ResolveRole` already uses. A second copy of the ceiling filter was rejected: two filters can drift apart. The dispatch plan itself is unchanged: only the log line reads the walk.

## Verification
The TDD tests and the cycle 1926 ACS suite pass. The full `runner`, `bridgechain`, `cliroute` and `cmd/evolve` packages pass with `-count=1`.

## Compatibility
The log format keeps its fields. The skip line is new. A bench that ends today keeps the short clock form.

## Limitations
The walk uses the plan's model tier. A plan whose first step-down tier differs from its model logs the model tier's walk.
