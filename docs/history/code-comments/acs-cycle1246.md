# Comment history: `acs/cycle1246`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1246/predicates_test.go:3` — above `package cycle1246`

```text
// Package cycle1246 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `reachabilityprobe-alias-resolution` (inbox item
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// What already landed: the reachabilityprobe library (cycle-1226), the frozen-pin
// derivation and its wiring into `evolve phase verify tdd` (cycle-1238). Triage
// dropped the todo-id's core scope as already-shipped and committed exactly one
// residual: resolvePackage (frozenpins.go:256-271) resolves the bare identifier
// written at a pinned call site by matching path.Base(pkg) == ident. An
// identifier introduced by an IMPORT ALIAS matches no package's base name, so
// the pin resolves to nothing and is silently skipped — a genuine cycle-644
// shape written as
//
//	import st "example.com/fixture/internal/storage"
//	... FileContains(t, "go/internal/core/state.go", "st.UpdateStateMap(")
//
// fails OPEN today, and the permanently unsatisfiable acceptance criterion sails
// through the gate that exists to catch it.
//
// Where the alias binding can live — the design constraint Builder inherits:
// NOT in the pinned production file. If core/state.go already imported storage
// while storage imports core, the module is already cyclic, `go list` refuses to
// produce a graph, and the gate fails open on infra ambiguity by design. The one
// place the binding can exist while the module still lists cleanly is the FROZEN
// TEST FILE carrying the pin (`go list -deps -json` ignores _test.go imports) —
// which is exactly where a structural test that also exercises the symbol
// declares it. So: resolve aliases from the import block of the frozen test file
// the pin was extracted from.
//
// Predicate strategy — every predicate exercises a REAL production path, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 drive the REAL operator entry point: a freshly built `evolve`
//     binary running `phase verify tdd --workspace ... --worktree ...` over a
//     real fixture Go module. This is House Rule 2's wiring proof — the library
//     fix must be reachable from the production caller
//     (internal/cli/phasecmd/phase_verify.go:141 withFrozenPinViolations), not
//     only from a unit test.
//   - 001 is the crux REJECTION case (aliased cycle-644 shape must be flagged).
//   - 002 is the false-positive guard: an aliased pin at a package that imports
//     nothing back is buildable and must still pass. Without it, a resolver that
//     flagged every aliased-or-unresolvable identifier would satisfy 001 and turn
//     the tdd gate into a false-HALT generator.
//   - 003 is the edge / fail-open axis: an identifier no import binds, and a
//     blank import (which binds no usable identifier), must both leave the
//     verdict untouched.
//   - 004 requires the DURABLE guard — the acs predicates here vanish with this
//     cycle; go/internal/reachabilityprobe/frozenpins_test.go is what keeps the
//     fix from silently rotting in a later refactor.
//   - 005 is House Rule 1's second half: ./internal/reachabilityprobe is already
//     enrolled in go/.apicover-enforce (line 495), so any NEW exported symbol
//     must be named and executed in its apicover_named_test.go.
//
// RED at authoring time is behavioural, not a compile failure: no new exported
// symbol is required (the fix is internal to resolvePackage/extractPins), so
// 001 and 004 fail on the verdict the CLI and the durable test actually produce
// today.
```

### `go/acs/cycle1246/predicates_test.go:142` — above `func fixtureWorktree(t *testing.T) string {`

```text
// fixtureWorktree builds a throwaway worktree whose go/ subdirectory is a real,
// `go list`-resolvable module carrying both shapes the gate must tell apart:
//
//	internal/storage  imports internal/core  → pinning it inside a core file is
//	                                           the cycle-644 shape (a cycle).
//	internal/leafutil imports nothing        → pinning it inside a core file
//	                                           closes no cycle.
//
// Every frozen test file pins a call site into go/internal/core/state.go; they
// differ only in how the referenced package's identifier is bound. The pins are
// REQUIREMENTS on production code, not calls the fixture itself makes, so the
// fixture module stays buildable and `go list` stays clean.
```

### `go/acs/cycle1246/predicates_test.go:169` — above `writeFile(t, wt, aliasCyclicFrozenTest,`

```text
// ALIASED cycle-644 shape: `st` -> internal/storage, bound in this frozen
// test file's own import block.
```

### `go/acs/cycle1246/predicates_test.go:261` — above `func TestC1246_001_AliasedPinFlaggedOnLiveCLI(t *testing.T) {`

```text
// TestC1246_001_AliasedPinFlaggedOnLiveCLI is the CRUX and the negative
// (rejection) predicate, asserted on the live production path: a cycle-644 shape
// whose referenced package is named through an import alias must be a CONFIRMED
// violation, exactly as the unaliased spelling already is.
//
// RED today: resolvePackage cannot map `st` to any package, returns ok=false,
// CheckFrozenPins skips the pin, and `evolve phase verify tdd` exits 0 on a
// permanently unsatisfiable acceptance criterion.
```
