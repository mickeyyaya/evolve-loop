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

## Invariants

- `guards.json` holds at most `max_rows` rows. `DefaultConfig` refuses more.
- One finding exists for each test and rule, however many times the rule matches in the test.
- A call that receives the test handle counts as an assertion. Shared helpers are not followed.
- The package imports the standard library only.

## Callers

No production code calls this package yet. The `evolve test audit` command that calls `Analyze` and `CheckShard` is the next component. Only tests reach the package today.

## Known limits

- The `unverified-fake` rule for an installed fake tool uses the config rows only. Two fixtures of cycle 2489 give the same source and the same config to the red case and the green case. No rule can tell them apart. The report of cycle 2489 records this as a predicate conflict.
