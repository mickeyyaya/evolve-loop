# Comment history: `acs/cycle1566`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1566/predicates_test.go:3` — above `package cycle1566`

```text
// Package cycle1566 carries the red-first-deliverable-reds-main lane's
// acceptance predicates for the two triage-committed tasks:
// `ship-new-test-red-gate` (the ship-time consumer for a newly added failing
// test) and `ship-new-test-red-production-wiring` (proof that the consumer is
// reached from the real production ship path, not merely from a helper).
//
// The lane's mechanism exists in the tree; cycle-1559's audit reproduced four
// defects in it against the real gate, and they are OPEN in
// `.evolve/runs/cycle-1566/defect-dispositions.json`. The predicates therefore
// split into pre-existing coverage (001-005, GREEN at RED time — pinned so the
// repair cannot weaken what already works) and this cycle's gaps (006-010, RED
// at RED time — one per open defect).
//
// H2 (007) is not hypothetical here: THIS file is a lone `//go:build acs`
// package newly added to this cycle's own shipping diff. Under the current
// gate it reports `[build failed]` and hard-blocks the lane's own honest ship
// with a false CodeRepoContractGate. The predicate that fixes the defect is
// the predicate that un-blocks its own ship.
//
// Predicate strategy — behavioral, never source-grep (cycle-85 ban): every
// predicate DRIVES the system by running the named Go test in ONE named
// package with `-run` narrowing, and requires a `--- PASS:` line for each
// name. Both halves are load-bearing: a non-matching `-run` pattern exits 0
// with "no tests to run", so exit-code-only checking would pass on an EMPTY
// repo, and a PASS line cannot appear if the package fails to build.
//
// Predicate map:
//
//	001 — T1 AC1 pre-existing: a newly added failing test blocks the ship
//	002 — T1 AC2 NEW: an explicit t.Skip reproducer does not block, at the gate
//	003 — T1 AC3 pre-existing: selection bounded to newly ADDED test files
//	004 — T2 AC1 pre-existing WIRING PROOF: runNative stops before git/ship
//	005 — T2 AC2 pre-existing: an honest skip is not blocked on that same path
//	006 — H1 NEW: a tag-guarded FAILING added test is not silently green
//	007 — H2 NEW: a tag-guarded GREEN added package is not a false RED
//	008 — H3 NEW: a string literal is not a build constraint
//	009 — M1 NEW: a failed discovery is recorded, never a silent disable
//	010 — M2 NEW: red messages are attributable to the scan that produced them
//	011 — anti-weakening: the four fixed-pack behaviours stay intact
```

### `go/acs/cycle1566/predicates_test.go:118` — above `func TestC1566_006_TaggedFailingAddedTestIsNotSilentlyGreen(t *testing.T) {`

```text
// TestC1566_006_TaggedFailingAddedTestIsNotSilentlyGreen — H1, incident
// 25040cea's shape: a `//go:build integration` failing test compiled out of an
// untagged backstop run ships as green.
```
