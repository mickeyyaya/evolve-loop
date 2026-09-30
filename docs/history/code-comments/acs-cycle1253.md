# Comment history: `acs/cycle1253`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1253/predicates_test.go:3` — above `package cycle1253`

```text
// Package cycle1253 materialises the cycle-1253 acceptance criteria for the one
// task this fleet lane committed: `tia-importer-closure` (triage top_n; the two
// sibling tasks `acssuite-scoped-regression-selection` and
// `tia-selection-wiring-proof` are DEFERRED and carry ZERO predicates here, per
// the R9.3 floor-binding rule).
//
// The task. changedpkgs derives changed-package selection FORWARD-ONLY: a
// changed file maps to the package it lives in, never to packages that import
// it. That is the cycle-1250 miss — a change confined to `internal/router` never
// selected `internal/routingtest`, which owns the keystone parity invariant, and
// main stayed red for 5 commits. Builder adds
// `changedpkgs.ImporterClosure(repoRoot string, pkgs []string) []string`: the
// sorted, deduped union of the input patterns and every module package that
// TRANSITIVELY imports one of them, best-effort (bad repoRoot / go-list failure
// → input unchanged, never a panic, never a lost entry).
//
// Predicate strategy — every predicate below EXECUTES the system under test via
// its unit suite in a single named package (`./internal/changedpkgs`), never a
// source-grep of production code (the cycle-85 degenerate-predicate ban) and
// never a `/...` test sweep (the flaky-predicate-shape ban):
//
//   - 001 is the crux: the cycle-1250 reproducer (router → routingtest) PLUS the
//     anti-no-op negative (a return-everything closure must fail).
//   - 002 pins transitivity (2-hop), the best-effort/no-panic contract, and the
//     sorted+deduped output shape.
//   - 003 is the apicover two-signal check scoped to this one package: the new
//     export must be NAMED and genuinely EXERCISED (non-zero coverage), not
//     merely present. `internal/changedpkgs` is enrolled in go/.apicover-enforce
//     (line 35), so an uncovered new export reddens the repo-wide gate.
//   - 004 is the blast-radius guard: the module still builds and the changed
//     package vets clean.
//
// Named unit tests are asserted by their `--- PASS:` line, so a renamed, skipped,
// or never-authored test cannot satisfy a predicate by a green package exit.
```

### `go/acs/cycle1253/predicates_test.go:74` — above `func TestC1253_001_ImporterClosureReproducer(t *testing.T) {`

```text
// -----------------------------------------------------------------------------
// AC1 + AC2 — the cycle-1250 reproducer, and the anti-no-op negative.
// -----------------------------------------------------------------------------
```
