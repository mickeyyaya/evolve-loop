# internal/testrefs

## Purpose

`internal/testrefs` proves that each test that a doc of record names is declared in Go. A doc that says "pinned by" a test name for an absent test is a false proof. The package finds each such citation and fails it, unless a shrink-only baseline lists it.

## Design

- **Defined names.** `Defined` walks a Go module root and reads each `_test.go` file with `go/parser`. It collects each top-level `func Test<Name>(` that has no receiver, so a name inside a string literal is not a declaration. Build-tagged files are read. The walk skips `vendor`, `testdata`, and directories that start with `.` or `_`. A file that does not parse is an error.
- **Cited names.** `Cites` reads each `.md` file under `docs/architecture`, `docs/incidents`, `docs/conventions`, `docs/operations` and `skills`, and each `.md` file directly in `agents`. Plans, research and `docs/private` are not docs of record. A citation is a backticked token that is a whole test name: `Test`, then a capital letter, a digit or `_`, then name characters. A `/subtest` suffix is dropped. A word such as `Testable` is not a citation.
- **Scope table.** One table lists each scope as a directory and a walk mode. The recursive mode reads the whole tree. The flat mode reads only the files directly in the directory. `Cites` has one loop over the table.
- **Generated regions.** A region between `<!-- GENERATED:... BEGIN` and `<!-- GENERATED:... END` is a copy of another doc. `Cites` skips it and the two marker lines, so the source is checked once. `stelint.RegionEdge` reads the markers. It is the one reader of this grammar.
- **Prefix patterns.** A name that ends in `*` or `_` is a prefix pattern. At least one declared name must start with the prefix.
- **Missing.** `Missing` returns, per doc, the sorted and unique cited names that resolve to no declared name.
- **Baseline.** `baseline.json` maps a doc path to the sorted names that are missing today. `Check` fails on a missing name that the baseline does not list. It also fails on a baseline row that now resolves or is no longer cited. A stale row is removed, so the baseline only shrinks. Never add a row to make a new miss pass: write the test or fix the name.
- **Base comparison.** `CheckAgainstBase` reads `baseline.json` at the merge base and on disk. It returns one sorted line for each (doc, name) row that the disk holds and the base did not hold. When the base has no baseline, it returns nothing. The build floor runs it (`testrefsFloorFailures`), so one diff cannot add a false citation and its baseline row together.
- **One scan.** `Scan` reads a repository root, runs the steps above and returns the sorted problems. A stale-row problem names the row. A person removes the row by hand.

## Invariants

- **The problem list is sorted and repeatable.** Pinned by `TestCheck_ANewMissFailsAndAStaleRowFailsAndSharedRowsPass`, which asserts sorted output.
- **The baseline only shrinks.** Pinned by `TestCheckAgainstBase_RefusesARowThatTheBaseDidNotHold` and, in the build floor, by `TestTestrefsFloorFailures_RefusesABaselineRowThatTheBaseDidNotHold`.
- **A name in a string is not declared.** Pinned by `TestDefined_ReadsDeclarationsOnlyAndSkipsFixtureDirs`.
- **A generated region is not read.** Pinned by `TestCites_FindsBackticksInTheDocsOfRecordOnly`.
- **The real tree holds the contract.** `TestDocsOfRecord_EveryNamedTestExists` in `internal/repocontract` runs `Scan` on the checked-in tree. `internal/repocontract` is in the scanner pack, so the test runs for each change.
