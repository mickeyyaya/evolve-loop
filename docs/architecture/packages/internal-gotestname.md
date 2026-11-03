# internal/gotestname

## Purpose

`internal/gotestname` holds two rules of the `go test` tool. The first rule says which function names are tests. The second rule says which directories the go tool ignores. Before this package, three guards had their own copy of the name rule, and the copies did not agree.

Each check that reads the syntax tree gets these rules from this package. On 2026-11-03 the copies of those checks moved here. Before that change, twelve walks kept a copy of the directory rule and seven checks kept a copy of the name rule. Ten of the directory copies and six of the name copies did not agree with this package.

Four text scanners still match test names with a regular expression. They are `testFuncRE` in `internal/inboxcurator/testindex.go`, the `git grep` pattern in `internal/inboxcurator/gitlog.go`, `testNameRE` in `internal/inboxcurator/landing.go` and `predicateTest` in `internal/continuation/repair_first.go`. They accept a name such as `Testify`. They need a migration of their own.

## Design

- **`HasPrefix` is the name rule for one prefix.** A name is a test when it is the prefix, or the prefix and a character that is not a lower-case letter. So `Test`, `TestA` and `Test_x` are tests, and `Testify` is not.
- **`Is` applies the rule to the three prefixes.** The prefixes are `Test`, `Fuzz` and `Benchmark`. `Example` is not a prefix, because an example has no body to check.
- **`IgnoredDir` is the directory rule.** The go tool ignores `testdata`, `vendor` and each name that starts with `.` or `_`.
- **`UnderIgnoredDir` applies the directory rule to a file path.** It checks each directory of the path. The `.` of a relative path is not a directory that the go tool ignores.
- **A walk keeps its root.** A caller that walks a tree checks `IgnoredDir` only below the root. Thus a walk that starts at `.` or at a directory named `vendor` still reads the root.
- **The package is a leaf.** It imports only the standard library, so a guard under `acs/` and a package under `internal/` can both use it.

## Consumers

The name rule:

- `acs/regression/emptytests`: the empty-test guard of ADR-0158 decision 19.
- `acs/regression/acsnames`: the `Test` and `Fuzz` functions of the index of declared tests.
- `internal/ciparity`: the `Test` functions of the e2e files.
- `internal/handoff/evidence`: a `test` evidence reference resolves only when its name is a test.
- `internal/handoff/request`: the predicates of a TDD to build request.
- `internal/acssuite`: the inventory of declared predicates, and the scope lint.
- `internal/paralleltest`: the top-level tests that the parallel guard judges.
- `internal/evalgate`: the floor predicates of the floor-binding gate.
- `internal/acspathscan`: the test functions whose file reads the scan collects.

The directory rule:

- `acs/regression/emptytests` and `acs/regression/acsnames`: the scans of the test files and of the `acs` tree.
- `internal/ciparity`: the e2e scan and the scan of the darwin set.
- `internal/coversplit`, `internal/sizeratchet`, `internal/rawgitratchet`, `internal/coverstrict`, `internal/codeshape`, `internal/commentaudit` and `internal/triagecap`: their module walks.
- `test/structure`: `SourceFiles`.
- `cmd/evolve`: the directory walk of `evolve docs ste-lint`.

## Lessons

- Seven copies of the name rule used a plain prefix match or a regular expression. Six of them accepted `Testify`, which `go test` never runs, and now refuse it. The copy in the `acssuite` inventory already refused it.
- **`..` counts as ignored.** `UnderIgnoredDir("../a.go")` is true, because `..` starts with `.`. The old segment copies gave the same result. No caller passes a path that leaves the tree.
- The evidence copy used a plain prefix match. It accepted `Testify`, which `go test` never runs. With this package, a reference to such a name does not resolve.
