# Comment history: `acs/cycle1323`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1323/predicates_test.go:3` — above `package cycle1323`

```text
// Package cycle1323 materialises the cycle-1323 acceptance criteria for the one
// fleet-scoped task pinned to this lane: `auto-refresh-binary-at-boundary`.
//
// Cycle 1323 is a CONTINUATION of cycle 1320. The boundary-refresh sequence
// (ahead-check -> rebuild -> repin -> ledger -> re-exec) already exists in
// go/cmd/evolve/cmd_loop_chain.go; cycle 1320's audit FAILed it with 9 defects,
// of which 2 (D1 short-sha ahead-check, D2 short-sha test gap) are already fixed
// in this tree. This cycle's acceptance bar is the remaining OPEN set:
//
//	AC1  dd8a8d64 / dcaf44e4 — the repin's ProvenanceVerified control is a real
//	     git-ancestor check, not the `func(string) bool { return true }` stub
//	     that stamps Authorized="provenance" on an unverified pin (ADR-0072
//	     forged-verdict class).
//	AC2  d7542cf6 — the repin pins sha256(<root>/go/bin/evolve), the binary the
//	     rebuild just produced, not selfsha.Running() (the stale running image).
//	AC3  ddb8f717 — the re-exec targets that same rebuilt binary, not
//	     exec.LookPath(os.Args[0]).
//	AC4  de8b9e49 / df20cf48 — an on-disk loop breaker caps re-execs for one
//	     unchanged build commit, so an ahead-check false positive degrades to
//	     "run batches on the current binary" instead of bricking the chain at
//	     zero batches executed.
//	AC5  d9d245d4 — the dead `repinCommit = "boundary-refresh"` sentinel is gone
//	     and can never be laundered past the provenance gate.
//
// Predicate strategy. Every predicate below EXERCISES the production function
// (`maybeRefreshChainBoundary`) or its production caller (`runLoopChain`) by
// running the named RED tests in go/cmd/evolve — never a source-grep of
// production text (the cycle-85 degenerate-predicate ban): a predicate that only
// asserted "cmd_loop_chain.go no longer contains `return true`" would pass on a
// cosmetic rename. Each run is narrowed with `-run` to exactly the tests that
// drive the changed seam (the flaky-predicate rule against whole-package
// ./cmd/evolve sweeps, cycles 1173/1175/1178) and demands an explicit
// `--- PASS: <name>` line per test, so a rename or a skip can never satisfy a
// predicate on exit code alone.
```

### `go/acs/cycle1323/predicates_test.go:132` — above `func TestC1323_005_sentinel_removed_and_prior_contract_intact(t *testing.T) {`

```text
// -----------------------------------------------------------------------------
// AC5 — the dead "boundary-refresh" sentinel commit is gone, plus a no-regression
// guard over cycle-1320's surviving boundary-refresh contract (the ahead-check,
// the ordering, the ledger, and the fail-open degrades must all still hold after
// this cycle's rework of the repin/re-exec path).
// -----------------------------------------------------------------------------
```

### `go/acs/cycle1323/predicates_test.go:144` — above `runChainTests(t,`

```text
// No-regression: every boundary-refresh test cycle 1320 left GREEN must
// still be GREEN after the repin/re-exec rework.
```
