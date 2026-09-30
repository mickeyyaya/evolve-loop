# Comment history: `acs/cycle1259`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle1259/predicates_test.go:3` — above `package cycle1259`

```text
// Package cycle1259 materialises the cycle-1259 acceptance criteria for the one
// task this fleet lane committed in triage `## top_n`:
//
//	wire-importerclosure-into-acssuite-scope (M — predicates 001-003)
//
// Both deferred items (`ciparity-digest-importerclosure-wiring`,
// `egps-regression-tia-selection-full-design`) carry NO predicates, per the R9.3
// floor-binding rule: a predicate may only gate work this cycle committed to.
//
// # The task
//
// `changedpkgs.ImporterClosure` (importerclosure.go:42) landed in cycle 1253 to
// close the cycle-1250 reverse-dependency blind spot — a change confined to
// `internal/router` never selects `internal/routingtest`, which imports router
// and owns the keystone parity invariant (main stayed red for 5 commits). The
// function is unit-tested green and WIRED NOWHERE: `grep -rn ImporterClosure`
// across go/ hits only its own definition, its unit tests, and the cycle-1253
// predicate that checks it compiles. Every real consumer still calls the
// forward-only derivations — including `changedPackagesForCycle`
// (acssuite.go:716), the function that populates CHANGED_PACKAGES, the env var
// EGPS Go predicates scope `go test` to. The fix for the miss exists, compiles,
// and is dead in production; that is why the class recurred a 3rd time
// (inbox item egps-regression-tia-selection, P1, weight 0.91).
//
// # Predicate strategy
//
// Every predicate drives the EXPORTED production entry `acssuite.Run` through
// its `Options.GoExec` seam and asserts on the CHANGED_PACKAGES value Run
// actually hands the predicate env — the same env every real predicate process
// inherits. This is the wiring proof: widening the unexported helper matters
// only if the widened value REACHES the env, and a test that called a helper
// directly would pass on a wiring that never gets there. No predicate greps
// production source (the cycle-85 degenerate-predicate ban), sweeps `/...`, or
// shells a 40s+ suite (the flaky-shape ban; measured baselines are
// internal/acssuite 1.20s, internal/changedpkgs 2.0s).
//
// Fixture roots are a temp dir whose `go` entry is a SYMLINK to the real module,
// so ImporterClosure has the real import graph to trace (the router→routingtest
// edge IS the reproducer) while the `.evolve/runs/` handoff — the other half of
// what projectRoot means to the gate runner — stays inside the temp dir. No
// predicate writes runtime state into the repository.
//
//   - 001 is the crux: the cycle-1250 reproducer through the production env,
//     plus its anti-no-op negatives (a blanket `./...` or a return-everything
//     widening must FAIL).
//   - 002 pins the SECOND handoff branch (handoff-builder.json): a widening
//     applied at one return site only is the same defect, half-fixed.
//   - 003 is the never-narrow floor plus the no-regression check on the two
//     packages the change touches.
//
// # BLOCKER — read the test-report before implementing
//
// `go/internal/acssuite/` is a PROTECTED CONTROL-PLANE SURFACE
// (guards.ProtectedSurfaceManifest, "the gate runner"). The role guard denies
// Edit/Write there from EVERY in-cycle phase, so the production one-liner these
// predicates demand cannot be landed by this cycle's Builder; it requires
// human-gated `evolve ship --class manual` outside a cycle. These predicates are
// the correct, executable RED contract for that operator change — they are
// expected to stay RED until it lands.
```

### `go/acs/cycle1259/predicates_test.go:78` — above `const (`

```text
// The expected CHANGED_PACKAGES entries are DERIVED from the same production
// mapping that produces them (changedpkgs.FileToPackage), never hand-written as
// pattern literals: the SSOT decides the form, and the predicate cannot drift
// from it. routerSourceRel is the fixture's only changed file; routingtest is
// its reverse dependent (the cycle-1250 edge); gitexec is a leaf that cannot
// transitively import router — the negative probe for a return-everything
// widening; and a module-root .go file maps to the blanket pattern, the other
// shape a no-op widening takes.
```

### `go/acs/cycle1259/predicates_test.go:175` — above `func TestC1259_001_ReverseDependencyReachesPredicateEnv(t *testing.T) {`

```text
// -----------------------------------------------------------------------------
// AC1 — the cycle-1250 reproducer through the production predicate env, and its
// anti-no-op negatives.
// -----------------------------------------------------------------------------
```

### `go/acs/cycle1259/predicates_test.go:180` — above `func TestC1259_001_ReverseDependencyReachesPredicateEnv(t *testing.T) {`

```text
// TestC1259_001_ReverseDependencyReachesPredicateEnv is the crux predicate.
//
// A cycle whose handoff touches only `internal/router` MUST export a
// CHANGED_PACKAGES that also names `internal/routingtest` — the package that
// imports router and holds the parity invariant a forward-only set never runs.
// The changed package itself must survive alongside it: widening never trades
// one package for another.
//
// The two negatives ride in the same predicate deliberately. A widening to the
// blanket `./...`, or one that returns every module package, satisfies the
// reproducer while making CHANGED_PACKAGES worthless — predicates scope
// `go test` to it, so an over-wide set puts them back to sweeping the whole repo
// (cycle-200, the reason the env var exists at all). gitexec is a leaf that
// cannot transitively import router, so it must stay OUT.
```
