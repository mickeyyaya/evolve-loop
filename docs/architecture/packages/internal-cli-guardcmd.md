# internal/cli/guardcmd

> This page is filled incrementally by handler. It now covers `evolve eval` (`eval.go`). Operator commands, flags and exit codes: [runtime-reference.md](../../operations/runtime-reference.md). The checks `evolve eval` runs: [internal-evalqualitycheck.md](internal-evalqualitycheck.md). The pipeline gates that run the same checks without the CLI: [internal-evalgate.md](internal-evalgate.md). This package is protected control-plane surface (`guards.IsProtectedSurface`, [ADR-0064](../adr/0064-pipeline-integrity-boundary.md)), so a loop lane may not edit it; changes land from the console.

## Purpose

`internal/cli/guardcmd` holds the `evolve` CLI handlers for the trust-kernel guard and pre-commit-gate subcommands: `guard`, `commit-gate`, `commit-prefix-gate`, `eval`, `preflight-environment` and `postedit-validate`. Each exported `Run*` function has the standard subcommand signature and is wired into the dispatcher table in `cmd/evolve/registry.go`. The handlers parse their own flags and delegate the work to the `internal/*` gate packages.

## `evolve eval`

`RunEval` dispatches `quality-check` (one eval's Level-0 tautology check, plus the predicate lints under `-predicates`), `diversity-check <evalsDir>` (the suite-level adversarial-diversity check) and `verify <eval.md> <workspace>` (independent re-execution of an eval). `quality-check` and `diversity-check` exit 0 for PASS, 1 for WARN or an internal error, 2 for HALT and 10 for bad arguments, the contract the scripts they replaced had.

- **`quality-check [-predicates <path>] <eval.md>`** grades one eval with `evalqualitycheck.Check` (Level-0 tautology detection). With `-predicates`, naming a `predicates_test.go` file or a `go/acs/cycle<N>` directory, it also runs the two advisory predicate lints over those sources: the flaky-shape lint (`flaky[...]` lines) and the unsatisfiable-predicate lint (`unsatisfiable[...]` lines).
- **Flags before or after the positional.** `parseInterspersed` collects positionals one at a time and re-parses the rest, because the standard `flag` package stops at the first positional and silently dropped `-predicates` in the documented `quality-check <eval.md> -predicates <dir>` order.
- **Keyed on the flag's presence.** The lints run whenever `-predicates` is given, even with an empty value (an unset shell variable), so the empty path reaches the lints and is reported as the error it is instead of skipping them silently.
- **One predicate-lint table, one path (Strategy).** `predicateLints()` lists each predicate lint by its receipt name and a strategy that runs it and phrases its findings; `predicateLint.advise` is the one path that reports them, so the receipt, the skip and the severity cannot drift between lints, and a third lint is a table row. Findings use a `flaky[...]` or `unsatisfiable[...]` prefix, deliberately not the `L<n>` prefix the tautology classifier uses, so an advisory note is never read as a claim that the eval is a weak tautology.
- **One receipt per lint, on every run.** Each lint's findings print one line each, then `[eval quality-check] <lint>: linted N file(s) under <path> — M advisory finding(s) (stage=advisory: raises PASS→WARN, never HALT)`, so "linted 0 file(s)" never reads as a clean tree.
- **A lint error is a loud, non-blocking skip, named once by its caller.** An unreadable path, an unparseable file or a directory with no `.go` files prints `evolve eval quality-check: <lint>: <error> (advisory lint skipped)` on stderr and contributes PASS. The name is the table's, the one the receipt prints (`flaky-lint`, `unsatisfiable-lint`); `internal/evalqualitycheck` returns its errors without a lint name, so each caller names the lint once in its own words (the gates say `flaky-shape lint`, `unsatisfiable-lint`). Pinned by `TestRunEval_QualityCheckPredicateLintSkipNamesEachLintOnce`.
- **Advisory severity joins monotonically.** Each lint returns WARN when it found anything and PASS otherwise, and `maxEvalLevel` joins it with the eval's own level. The join can raise PASS to WARN but never lower a tautology HALT, whichever lints are added.

## Invariants

- The unsatisfiable-predicate lint runs through the binary's real entry point: `TestDispatch_EvalQualityCheckPrintsTheUnsatisfiableLintReceipt` (`cmd/evolve`) drives `dispatch` with `eval quality-check <eval.md> -predicates <file>` and requires exit 1 and the `unsatisfiable-lint` receipt.
- `TestRunEval_QualityCheckPredicatesUnsatisfiableLint` pins WARN on findings (both an inverted primitive and an exit code through `go run`), HALT kept under findings, the receipt on a clean package, and the loud skip on an unreadable path. The flaky-shape lint's equivalents are `TestRunEval_QualityCheckPredicatesFlakyLint`, `TestRunEval_QualityCheckPredicatesReceiptIsUnconditional` and `TestRunEval_QualityCheckFlakyLintNeverLowersHalt`; the join itself is `TestMaxEvalLevel_IsMonotonic`.

## Findings

- **Cycle 1793 (2026-10-05)**: the lane building the unsatisfiable-predicate lint froze a predicate on the `unsatisfiable-lint` receipt, which only this package prints. Every tree a lane may produce failed either that predicate or the protected-surface floor, so the wiring and its tests landed from the console. A predicate whose observable output only a protected path produces is console work, never a lane contract.
