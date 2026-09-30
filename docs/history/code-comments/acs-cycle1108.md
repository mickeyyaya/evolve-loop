# Comment history: `acs/cycle1108`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1108/predicates_test.go:3` — above `package cycle1108`

```text
// Package cycle1108 materialises the cycle-1108 acceptance criteria for the
// single triage-committed top_n task of this lane:
// `gitstage-quotepath-determinism` (layer 3 of the ship staging onion).
//
// The defect: ship decides "what to stage" from `git status --porcelain` and
// `git check-ignore` output, and neither reader accounts for git's C-quoting.
// porcelainChangedPaths (manifest.go:198) strips wrapping quotes only, so
// `"caf\303\251.txt"` becomes a 15-byte escaped string that exists on no disk
// and matches no manifest entry; dropIgnoredPaths (gitops.go:801) trims
// whitespace only, so a quoted probe line never matches the raw declared path
// it is filtering and an ignored path survives into `git add` — the cycle-1101
// rc=1 ship-killer, narrowed to the non-ASCII/quote-bearing input class.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1104
// precedent). porcelainChangedPaths, dropIgnoredPaths and stageExplicitPaths
// are all UNEXPORTED in internal/phases/ship, so they cannot be imported from
// here; each predicate instead shells `go test -run` over the RED contract
// tests authored this cycle in go/internal/phases/ship. Every one of those
// CALLS the parser/filter directly or drives the real shipDirect staging path
// and asserts on returned values and on the argv git was actually invoked
// with. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban).
//
// RED now: porcelainChangedPaths does not unescape, the git reads carry no
// `-c core.quotePath=false`, and dropIgnoredPaths does not decode probe output.
```

### `go/acs/cycle1108/predicates_test.go:101` — above `func TestC1108_004_IgnoredProbeMatchesQuotedOutput(t *testing.T) {`

```text
// TestC1108_004_IgnoredProbeMatchesQuotedOutput — AC4, positive half. An ignored
// non-ASCII or quote-bearing declared path must be dropped from the pathspec
// even when `check-ignore` reports it C-quoted; otherwise it reaches `git add`,
// which exits 1 on ANY ignored pathspec and kills the ship (cycle-1101).
```

### `go/acs/cycle1108/predicates_test.go:128` — above `func TestC1108_006_ShipStagingContractStillGreen(t *testing.T) {`

```text
// TestC1108_006_ShipStagingContractStillGreen — AC5 anti-regression across the
// whole staging onion: the landed layer-1 (absolute pathspec, `d202aeb6`),
// layer-2 (ignored paths, `e8990e53`), explicit-pathspec (cycle-1067) and
// staged-deletion contracts must all still hold. This is the predicate that
// fails if Builder greens the quoting cases by reworking the shared staging
// path rather than extending it.
```
