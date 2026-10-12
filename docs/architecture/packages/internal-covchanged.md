# internal/covchanged

> The plan: [cover-changed-gate-2026-10.md](../../plans/cover-changed-gate-2026-10.md) (2026-10-11, fix round 1 on 2026-10-12). The verb: `evolve cover changed` ([runtime-reference.md](../../operations/runtime-reference.md)). The floor target: `make cover-changed` ([go/docs/testing.md](../../../go/docs/testing.md)).

## Purpose

`covchanged` finds each Go function that a change touches and measures it against a coverage profile. It refuses each changed function below 100% statement coverage. The verb `evolve cover changed`, the target `make cover-changed`, the CI step in `.github/workflows/go.yml` and the loop build floor (`go/internal/core/cover_changed_floor.go`) call the same `Workspace.Check`.

## Design

- **A pure core with ports.** `ParseDiff`, `ParseProfile`, `Measure` and `Judge` do no IO. `Measure` gets the changed lines, the profile, a file reader and a `go/build` context in `Sources`. `Workspace.Check` is the one adapter: it runs git through a `sysexec.RunFunc` and reads files from the module directory.
- **Code lines come from the tokens of the new file.** `go/scanner` gives the lines that hold a token. A changed line that holds no token (a blank line, a line comment, a line inside a block comment) does not make its function "changed". Thus a block comment that starts above the hunk is also ignored.
- **Removed code counts.** `Diff.Removed` holds the new-side line where a hunk removes code. A function holds that point when the point is from its first line up to the line before its last line. A hunk that removes only comments records nothing.
- **The diff is strict.** `ParseDiff` counts the lines of each hunk from its header. A line past the count, a line of no known kind, a bad header and a diff that ends inside a hunk are errors. A quoted path is unquoted. The diff turns rename detection off and gives paths relative to the module, so a moved file counts as changed.
- **The diff starts at the merge base.** `Check` diffs from the merge base of the base and `HEAD`. Thus a base that moved on after the branch started adds nothing.
- **Untracked files count.** `git ls-files -z --others --exclude-standard` gives the untracked `.go` files, and each of their lines counts as added. Thus a new file that the operator did not stage is measured, not skipped.
- **Blocks merge by key.** `ParseProfile` merges duplicate blocks (the same file and span from two test binaries) with an OR of the hit. A block counts in the function that holds its start, so a block that spans two functions counts once.
- **Exemptions are listed, not hidden.** `Report.Exempt` names each exempt file with its reason. A test file and a file under `testdata` or `vendor` are exempt. So is a file that the build constraints exclude for the host GOOS and GOARCH and `Options.Tags`. Generated files are not exempt.
- **"No profile" is a refusal.** A changed function with no block is not at 100%. A function with zero statements and a block is at 100%.
- **A stale profile stops the check.** The file reader refuses a changed file that is newer than the profile, and names `make -C go test-integration`.
- **One module-path reader.** `ModulePath` is exported. `internal/coverstrict` calls it, and `ParseProfile` is its one profile parser (`evolve cover strict`, plan T2).
- **The key.** `<module dir>.<Name>` or `<module dir>.<Receiver>.<Name>`, the same form as the size ratchet. The receiver forms `*T`, `T[K]` and `T[K, V]` all give `T`.

## Invariants

- **A comment-only change, a block comment and a blank line change no function** (`TestMeasure_CountsOnlyTheFunctionsAChangedCodeLineTouches`).
- **Removed code changes the function that held it, and a removed comment changes nothing** (`TestMeasure_RemovedCodeTouchesTheFunctionThatHeldIt`).
- **Every changed function below 100% is refused** (`TestJudge_RefusesEveryChangedFunctionBelowFullAndCountsTheRest`).
- **A real git change is refused by exactly its changed function, also when the base moved on** (`TestWorkspaceCheck_RealGit_RefusesExactlyTheChangedFunctionBelowFull`, tag `integration`).
- **The parsers never panic** (`FuzzParseDiff`, `FuzzParseProfileLine`). A profile line that parses gives the same result after it is printed and parsed again.
- **Enforced at 100%.** The package is in `go/.apicover-enforce` and in `go/.cover-strict` at 100.

## Findings

- **The first build had an allowance file** (2026-10-11). It listed the 192 gaps of the audit. Fix round 1 removed it, because it waived the operator rule for each lane that touched those functions.
- **The fixtures package is not usable here.** `test/fixtures` imports `internal/core`, and `internal/core` imports this package. Thus the tests use a small local git fake.
