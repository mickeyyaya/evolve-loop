# Comment history: `acs/cycle1507`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1507/predicates_test.go:3` — above `package cycle1507`

```text
// Package cycle1507 materialises the cycle-1507 acceptance criteria for the two
// fleet-scoped top_n tasks pinned to this lane
// (`park-consume-releases-continuation-binding`):
//
//   - transactional-registry-retire-on-park-consume — an item leaving the
//     pending pool (park/quarantine, ship-consume) MUST release its
//     continuation-registry binding in the SAME operation, with the binding
//     VALUE preserved into the item file (`released_continuations[]`) so the
//     salvage pointer survives the release.
//   - planner-and-adoption-live-scope-guard — the scope-keyed registry read
//     (`inboxmover.ResolveContinuationForScope`, the ONE seam both the wave
//     planner's lane-scope minting and the post-triage adoption path go
//     through: injected at cmd/evolve/cmd_cycle.go:711 into
//     core.WithContinuationResolver) MUST refuse a binding whose scope id has
//     no live pending item — logged, released — instead of re-dispatching a
//     parked/consumed scope forever (live burns: cycles 1487, 1497).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): every predicate
// here drives a REAL production function against an on-disk fixture and asserts
// on its return value / stderr / the resulting on-disk bytes. `inboxmover` and
// `continuation` are imported and called directly — same module, so these are
// the production symbols, not a re-implementation. The only source-text
// assertion in this file is explicitly auxiliary (predicate 003) and carries no
// verdict on its own.
//
// "Live pending item" is defined by the batch loader's own reach, so the guard
// and the dispatcher can never disagree: an id is LIVE iff a `.json` in the
// inbox ROOT (inboxbatch.LoadDir's non-recursive scan) or in
// `inbox/processing/cycle-*/` (a lane currently holding it) carries that id.
// consumed/, quarantine/, processed/, rejected/ and retry/ are NOT live —
// LoadDir skips subdirs, which is exactly why a parked item stops being picked.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no multi-package
// `go test`, no wall-clock deadline, no literal PID; every subprocess gets an
// explicit cmd.Dir, never process cwd. The two `go test` invocations name ONE
// package each and neither is a known 40s+ suite.
```

### `go/acs/cycle1507/predicates_test.go:241` — above `func TestC1507_004_AdoptionRefusesGhostScopeAndReleases(t *testing.T) {`

```text
// TestC1507_004_AdoptionRefusesGhostScopeAndReleases is the negative test and
// the load-bearing anti-no-op signal for task 2: a registry binding whose scope
// id has NO live pending item anywhere must NOT be handed back for dispatch —
// it is logged and released. This is the exact cycle-1487/1497 shape (item
// parked/consumed out of the pool, binding immortal, lane minted anyway).
```
