# pkg/acsassert

> The ACS predicates that import this package live under `go/acs/{cycle<N>,regression,redteam}/`. The retired bash predicates are in [ADR-0042](../adr/0042-egps-v11-go-native-predicates.md).

## Purpose

`pkg/acsassert` is the assertion DSL that Go ACS predicates write against. Each helper does the work that `[ -f ]`, `grep -q` and `jq -e` did in the retired bash predicates. The package is public. Its exported names are the contract of every predicate, so none is renamed or removed.

## Design

- **`TB`, not `*testing.T`.** The helpers take the minimal `TB` interface (log a failure, mark a helper). Their own tests drive them with a fake and check the failing behavior, and the outer test does not fail.
- **Two reader families.** The value-only readers are `FileContainsAny`, `CountOccurrencesAny` and `LineContainsAll`. They return a bool or a count and take no `TB`, so the caller controls the failure mode. Each has a `...Checked` sibling that returns an error. On a missing path the error names the path and the moved-file hint, and it wraps `fs.ErrNotExist`.
- **Equality does not panic.** `JSONFieldEquals` compares with `reflect.DeepEqual` (`equalAny`). A dot path that resolves to a map or slice is compared and does not panic.
- **Source parsing ignores comments.** `CountInGoFunc` parses with `parser.ParseFile(..., 0)`. No comment is read or kept as a node, and nothing in the package parses comments.
- **`RepoRoot` skips outside a git work tree.** Predicate suites then run cleanly on bare exports, with no false failures.
- **`GoTests` judges the events of `go test -json`.** For each name in `Names`, it requires one `run` event and then one `pass` event. A pattern made of `^Name$` and `^(A|B)$` terms, joined by `|`, names exact tests, and `Names` must list each one. The spec is [acs-predicate-quality-gate.md](../acs-predicate-quality-gate.md#structured-go-test-evidence-acsassertgotests-2026-09-28).

## Invariants

- **An absence assertion uses `FileNotContains`.** `FileContains` is a positive assertion. It calls `Errorf` "missing" when the substring is absent. The idiom `if FileContains(...) { Errorf("present") }` thus fails on the correct (absent) state, because the inner `Errorf` fires and the `if` body is skipped. The `FileNotContains` tests in `assertions_test.go` pin this.
- **A missing file or function is a loud error, never a silent zero.** A bool-only reader cannot tell "file missing" from "content absent". A count of 0 cannot tell "function renamed" from "no occurrences". The `...Checked` forms and `CountInGoFunc` return an error for a missing file, unparsable source or a missing function. Thus a renamed function fails an `==0` predicate and does not satisfy it. `TestCountInGoFunc` and the `...Checked` tests pin this.
- **Function-scoped counting resists rot.** File-wide counts (`CountOccurrencesAny == N`) break when a later change adds an occurrence elsewhere in the file. `CountInGoFunc` counts only inside the named top-level function. A method matches by its bare name, and the receiver is ignored. On a name collision the first declaration in the file wins. `TestCountInGoFunc_MethodReceiver` pins this.
- **A line counts once.** `CountOccurrencesAny` counts a line that matches several variants one time.
- **`GoTests` refuses an exact pattern name that `Names` omits.** A name outside `Names` is never checked, so a renamed test passes by omission. `unlistedExactNames` compares the exact names of the pattern with `Names` before the run starts. `TestExactNames_ReadsOnlyFullyAnchoredLists`, `TestUnlistedExactNames_ReportsEachExactNameThatNamesLacks`, `TestGoTests_RefusesAnExactPatternNameThatNamesDoNotList` and `TestGoTests_AStaleExactPatternNameCannotHideOutsideNames` pin this.

## Findings

- A predicate had a file-wide count that was refreshed from 1 to 2 within one day. That rot is the reason `CountInGoFunc` exists.
- Predicates that inverted `FileContains` were red on the correct state. That is the reason `FileNotContains` exists.
- A predicate that ran a test by name through `go test -run` passed when a rename or a delete removed the test. `go test` exits 0 on a name that matches nothing. `GoTests` now refuses an exact pattern name that `Names` omits. For the predicates that do not call `GoTests`, [the `acsnames` check](../acs-predicate-quality-gate.md#predicate-test-names-resolve-acsnames-2026-10-18) fails when a named test does not exist. On 2026-10-18 it found 20 stale names in 16 packages.
