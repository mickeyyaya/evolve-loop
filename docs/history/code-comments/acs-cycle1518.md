# Comment history: `acs/cycle1518`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1518/predicates_test.go:3` — above `package cycle1518`

```text
// Package cycle1518 materialises the cycle-1518 acceptance criteria for the
// single triage-committed task
// `promote-cycle1515-continuation-predicates-to-acs-regression`.
//
// Scope note (verified live, not assumed from the filed item). This lane's
// fleet_scope.todo_ids — `registry-release-on-park-consume` and
// `continuation-operator-cli` — are BOTH already implemented and green at this
// worktree's HEAD (`internal/inboxmover/continuation_retire.go`,
// `internal/phases/ship/consume.go`, `go/cmd/evolve/cmd_continuation.go` wired
// at `registry.go:73`). The scout/triage verdict is therefore a durability gap,
// not a functionality gap: the eight predicates that pin that shipped behaviour
// live in `go/acs/cycle1515/` — a PER-CYCLE ACS package that CI never sweeps.
// `.github/workflows/ci.yml`'s `acs-durable` job runs exactly
// `go test -count=1 -tags acs ./acs/regression/...`, so nothing under
// `go/acs/cycle1515/` is a standing gate. This cycle promotes that file into
// the durable path.
//
// Destination pinned by these predicates: `go/acs/regression/cycle1515/`,
// package `cycle1515`, with the eight `TestC1515_001..008` names carried over
// VERBATIM. Triage's files list illustrated the destination as
// `go/acs/regression/continuation_operator_cli/`; that path is NOT used, for two
// reasons stated here rather than decided silently: (1) fifteen sibling durable
// packages already use the `regression/cycle<N>` shape (cycle100, cycle1270,
// cycle85 …) and none uses an underscored topic name, and (2) keeping the
// package and test names identical makes the promotion a pure move, keeps
// `.evolve/evals/continuation-operator-cli.md`'s `-run '^TestC1515_00N'`
// evidence commands valid, and keeps the cycle-1515 provenance readable in CI
// output. Whether the ORIGINAL `go/acs/cycle1515/` copy is deleted or left in
// place as a historical record is deliberately NOT pinned — either is correct,
// Builder documents the call in build-report.md.
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): the load-bearing
// assertions here all drive the real Go toolchain against the real repository —
// `go test` actually EXECUTES the promoted predicates (which in turn build the
// `evolve` binary and drive the production inboxmover/continuation seams), and
// `go list` reports the real package/dependency graph. Adding text to a source
// file cannot satisfy any of 001-004. Predicate 005 is an inherent
// config-presence check on the CI workflow and carries the explicit waiver.
//
// Reliability (flaky-predicate-shape rules): every subprocess names exactly ONE
// package (never a `/...` sweep of test execution — the single `go list`
// pattern-expansion in 002 is a metadata query, not a test run), every
// invocation sets an explicit cmd.Dir or `git -C`, there is no wall-clock
// deadline, no literal PID and no un-reaped load generator.
```

### `go/acs/cycle1518/predicates_test.go:64` — above `var promotedTests = []string{`

```text
// promotedTests are the eight cycle-1515 predicates that must survive the
// promotion by name: 001-002 pin `registry-release-on-park-consume` (park
// releases the binding, preserves the pointer, invents no annotation),
// 003 pins the live-item guard, 004-008 pin the `continuation-operator-cli`
// surface. Losing any one of them silently narrows the durable gate.
```

### `go/acs/cycle1518/predicates_test.go:140` — above `func TestC1518_001_PromotedPredicatesRunGreenOnTheDurablePath(t *testing.T) {`

```text
// TestC1518_001_PromotedPredicatesRunGreenOnTheDurablePath EXECUTES the promoted
// package exactly the way the acs-durable CI job would reach it, and requires
// every one of the eight cycle-1515 predicates to report PASS. Running the suite
// is what makes this a behavioural predicate: the promoted predicates themselves
// build the `evolve` binary and drive the real inboxmover/continuation seams, so
// a copied-but-broken file, a file whose build tag excludes it, or a package that
// silently dropped predicates all fail here.
//
// The explicit PASS-name check is load-bearing anti-vacuity: `go test` on a
// package containing ZERO tests exits 0, so exit code alone would be satisfied
// by an empty stub file at the destination.
```
