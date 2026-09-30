# Comment history: `acs/cycle1144`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1144/predicates_test.go:3` — above `package cycle1144`

```text
// Package cycle1144 materialises the cycle-1144 acceptance criteria for this
// lane's two triage-committed top_n tasks:
//
//	docs-floor-architecture-class-gate   (M) — deterministic architecture-class
//	    classifier + a new `missing_architecture_docs` violation wired into the
//	    ADR-0034 `deliverable.Verify` seam, so the `evolve phase verify`
//	    self-check and the host-side reviewer gate enforce the docs floor with
//	    byte-identical logic.
//	docs-floor-backfill-fleet-landing    (S) — document `fleet.landing`
//	    (go/internal/policy/policy.go:1043-1208, values "per-lane" /
//	    "prefix-queue") in control-flags.md, runtime-reference.md and a
//	    standalone ADR: the first enforced instance of that floor, and the live
//	    proof the gap was real.
//
// Predicate strategy. Task 1's predicates are behavioural-via-subprocess (the
// cycle-549…1108 precedent): each shells `go test -run` over the RED contract
// tests authored this cycle in go/internal/deliverable/architecture_docs_test.go,
// every one of which CALLS ArchitectureDocsViolations / VerifyBuildWithChangedPaths
// and asserts on returned values — none is a source-grep of production code (the
// cycle-85 degenerate-predicate ban). Task 2 is a DOCUMENTATION deliverable: the
// doc file IS the artifact under test, so asserting on its emitted content is a
// real-artifact assertion, not a source-grep proxy. Docs are read under
// acsassert.RepoRoot (the WORKTREE — the dual-root pattern, go/acs/README.md),
// because a doc edited this cycle reaches main only at ship.
//
// RED now: ArchitectureDocsViolations / VerifyBuildWithChangedPaths /
// CodeMissingArchitectureDocs do not exist, so the deliverable package fails to
// compile; and `fleet.landing` appears in no doc.
```

### `go/acs/cycle1144/predicates_test.go:103` — above `func TestC1144_004_FloorIsWiredIntoTheVerifySeam(t *testing.T) {`

```text
// TestC1144_004_FloorIsWiredIntoTheVerifySeam — AC5, the WIRING proof. A
// classifier nobody calls is a no-op: the verdict must surface as a Result from
// the ADR-0034 verify seam (!OK, correct Phase, floor code present) on an
// otherwise perfectly well-formed build-report, and must be ADDITIVE — the
// pre-existing well-formedness violations still surface alongside it.
```
