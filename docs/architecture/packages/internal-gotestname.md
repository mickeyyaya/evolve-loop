# internal/gotestname

## Purpose

`internal/gotestname` holds two rules of the `go test` tool. The first rule says which function names are tests. The second rule says which directories the go tool ignores. Before this package, three guards had their own copy of the name rule, and the copies did not agree.

This package is not yet the one home of these rules. Other packages still keep their own copy of each rule. Two consumers below also keep a partial copy of the directory rule. The e2e scan of `internal/ciparity` and the scan of the `acs` files in `acs/regression/acsnames` skip only `testdata`. These copies need a migration. When you change a rule here, the remaining copies keep the old rule.

## Design

- **`HasPrefix` is the name rule for one prefix.** A name is a test when it is the prefix, or the prefix and a character that is not a lower-case letter. So `Test`, `TestA` and `Test_x` are tests, and `Testify` is not.
- **`Is` applies the rule to the three prefixes.** The prefixes are `Test`, `Fuzz` and `Benchmark`. `Example` is not a prefix, because an example has no body to check.
- **`IgnoredDir` is the directory rule.** The go tool ignores `testdata`, `vendor` and each name that starts with `.` or `_`.
- **The package is a leaf.** It imports only the standard library, so a guard under `acs/` and a package under `internal/` can both use it.

## Consumers

- `acs/regression/emptytests`: the empty-test guard of ADR-0158 decision 19.
- `acs/regression/acsnames`: the directories that the index of declared tests skips.
- `internal/ciparity`: the `Test` functions of the e2e files, and the directories that the scan of the darwin set skips.
- `internal/handoff/evidence`: a `test` evidence reference resolves only when its name is a test.

## Lessons

- The evidence copy used a plain prefix match. It accepted `Testify`, which `go test` never runs. With this package, a reference to such a name does not resolve.
