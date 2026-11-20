# internal/testaudit

## Purpose

`internal/testaudit` finds weak tests with `go/ast` rules. It compares the findings with a baseline file in each package, so the count of weak tests can only fall. It is component T1 of the test-quality audit.

## Design

- `Analyze` walks a root and parses each `_test.go` file. It runs 13 rules on each `TestXxx(t *testing.T)` function. `RuleIDs` lists the rules.
- The rules read the syntax tree, never the source text. A test name inside a raw string is a string literal, so it is never a finding.
- A finding is `package`, `test` and `rule`. It has no line number, so a moved test keeps its baseline row.
- A guard row exempts one test, or a whole package when `test` is empty. A guard row never exempts by the shape of the code.
- `fakes.json` lists each fake command-line tool. A row with real side `none` needs a reason. A row with a suite directory needs a `fakecontract.Suite` for its tool in that directory.
- `WriteShard` writes `testdata/testaudit-baseline.json` in a package. `CheckShard` fails on a finding with no baseline row. It also fails on a baseline row with no finding.
- `AnalyzeTree` walks a module root and emits clean relative package keys. `CheckTree` loads every package shard and `internal/testaudit/unwritable-baselines.json`, then fails on any missing or stale row.
- `CheckPackages` gives the same rows as `CheckTree`, but for a list of packages. It drops each package with a `testdata`, `vendor`, `.x` or `_x` path segment. It keeps a deleted package, so the central rows of that package show as stale. A red package outside the list is not reported.
- `packageFindings` and `packageBaseline` are the one projection of a package. `Rebaseline` uses `packageFindings`. `CheckPackages` uses both functions.
- `AuditScope` is the one rule that maps a changed path to a `Scope`. A `.go` file or a local shard gives its package. A change to `guards.json`, `fakes.json`, `unwritable-baselines.json` or a non-test `.go` file of this package sets `Whole`. Then only the full tree check can judge the change.
- `LoadConfig` reads `fakes.json` and `guards.json` from a module root. `DefaultConfig` reads the embedded copies. Both call `parseConfig`, the one step that parses and validates. A test pins `LoadConfig` of the real module equal to `DefaultConfig`.
- `RowError` is the typed error for one finding. Its kind is `RowMissing` (no baseline row) or `RowStale` (a row with no finding). It unwraps to `ErrNoBaselineRow` or `ErrStaleBaselineRow`. The message text is the same as before. `Problems` flattens a joined error into its parts.
- `Rebaseline` updates one package's shard or its entry in `internal/testaudit/unwritable-baselines.json`. It refuses any finding absent from an existing baseline, and it removes an empty local shard.

## Invariants

- `guards.json` holds at most `max_rows` rows. `DefaultConfig` refuses more.
- One finding exists for each test and rule, however many times the rule matches in the test.
- A call that receives the test handle counts as an assertion. Shared helpers are not followed.
- The package imports the standard library only.

## Callers

- `evolve test audit` (`go/cmd/evolve/cmd_test_audit.go`, registered in `go/cmd/evolve/registry.go`) calls `AnalyzeTree`, `CheckTree`, and `Rebaseline`.
- `testAuditFailures` in `go/internal/core/build_testaudit_check.go` calls `AuditScope`, `LoadConfig`, `CheckPackages` and `Problems` in the build floor.
- `TestTreeRatchet` (`go/internal/testaudit/ratchet_tree_test.go`) calls `CheckTree` on the module root during `go test ./internal/testaudit`.

## Known limits

- The `unverified-fake` rule for an installed fake tool uses the config rows only. Two fixtures of cycle 2489 give the same source and the same config to the red case and the green case. No rule can tell them apart. The report of cycle 2489 records this as a predicate conflict.
