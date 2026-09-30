# Comment history: `acs/cycle1469`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1469/predicates_test.go:3` — above `package cycle1469`

```text
// Package cycle1469 materialises the cycle-1469 acceptance criteria for the two
// top_n tasks pinned to this lane. Both are the same root cause — a git-output
// path reader in go/internal/phases/ship that was never enrolled in the
// cycle-1108 quote-path contract:
//
//   - gitstage-collider-quotepath  → detectColliders (gitops.go) reads
//     `git diff --name-only` and `git status --porcelain` without
//     `-c core.quotePath=false` and with a naive strip-the-outer-quotes
//     decode, so a C-quoted incoming filename compares as escaped text that
//     exists on no disk and the collider is invisible to the ff-merge
//     pre-flight AND to the repair ladder.
//   - gitstage-rename-arrow-parse  → porcelainChangedPaths / stagedGonePaths
//     (manifest.go) split the whole payload on every " -> ", but git quotes
//     rather than escapes a filename containing that sequence (verified
//     against git 2.50.1: `?? "we -> ird.txt"`, and `core.quotePath=false`
//     does NOT suppress it), so one rename is torn into 3-4 unbalanced-quote
//     fragments that `git add` rejects rc=128 — failing the ENTIRE staging.
//
// Predicate strategy. Both fixes live in UNEXPORTED functions of an internal
// package, so the only honest behavioural driver is the package's own test
// binary: each predicate shells ONE named package (`./internal/phases/ship`)
// narrowed with `-run` to the RED contract this cycle authored, and asserts on
// its exit code. That is the sanctioned shape — never a `./...` sweep, never a
// wall-clock bound, never a bare `git` (every git call is `-C <root>`). No
// predicate here source-greps production code: adding a magic string to
// gitops.go/manifest.go cannot make any of them pass, only a real decode/
// tokenizer change can.
//
// 003 is the anti-regression half: the pre-existing quote-path and staging
// contracts (cycle-1067/1101/1108) must still pass, so a tokenizer rewrite that
// buys the quoted-arrow case by breaking plain renames fails the cycle.
```

### `go/acs/cycle1469/predicates_test.go:51` — above `var redContractFiles = []string{`

```text
// redContractFiles are the RED contract this cycle authored. They must be
// present on disk AND shippable (not gitignored) — see assertShippable for why
// tracking itself is the wrong assertion at audit time (cycle-93).
```

### `go/acs/cycle1469/predicates_test.go:79` — above `func assertShippable(t *testing.T, root, rel string) {`

```text
// assertShippable is the cycle-93 pairing for a file-backed contract: the file
// must exist on disk AND be SHIPPABLE — i.e. not gitignored, since a gitignored
// test file is silently dropped at ship and the RED contract it encodes
// evaporates. Tracking itself is deliberately not asserted: predicates run at
// audit, before the ship stages the cycle's new files, so `ls-files` would
// false-RED on a legitimately new test file. `git check-ignore` exits 1 when
// the path is NOT ignored, which is the passing case.
```

### `go/acs/cycle1469/predicates_test.go:150` — above `func TestC1469_003_ExistingQuotePathAndStagingContractsHold(t *testing.T) {`

```text
// TestC1469_003_ExistingQuotePathAndStagingContractsHold — the anti-regression
// predicate, and the reason this cycle cannot be passed by loosening a parser.
// The cycle-1067 explicit-staging and cycle-1101/1108 quote-path contracts
// predate this cycle; a tokenizer or decode change that buys the quoted-arrow
// case by breaking plain renames, ASCII classification, or the ignored-path
// filter trades one rc=128 ship-killer for another and must fail here.
```
