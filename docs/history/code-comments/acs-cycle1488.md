# Comment history: `acs/cycle1488`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1488/predicates_test.go:3` — above `package cycle1488`

```text
// Package cycle1488 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `verdict-cache-fresh-base-collision`.
//
// State of the tree at RED. The inline fresh-base guard already exists at BOTH
// verdict-cache call sites (orchestrator.go's pre-loop ADR-0048 shadow probe and
// phase_bindings.go's audit-binding Put), and the pre-existing regression
// TestVerdictCacheCollisionRegression is GREEN. What does NOT exist is the
// deterministic, exported eligibility predicate the scout report calls for — the
// guard is a comparison duplicated at two sites, so a future enforce-stage
// lookup has nothing to reuse and the two copies can drift independently. The
// cycle-1488 bar is therefore single-sourcing, not re-fixing: expose
// verdictcache.ProbeEligible(baseTreeSHA, candidateTreeSHA) and route both call
// sites through it without changing observed behaviour.
//
// Empty-base semantics are pinned to what SHIPPED, not to a fresh reading of the
// scout text: an empty/unresolvable base tree keeps the candidate ELIGIBLE
// (TestVerdictCacheCollisionRegression's "missing base remains lookup eligible"
// row is frozen). Only an empty CANDIDATE is rejected outright — it carries no
// content identity, matching Lookup/Put's existing empty-SHA no-op.
//
// Predicate strategy (cycle-85 degenerate-predicate ban):
//   - 001 CALLS the predicate directly over a positive/negative/edge table.
//   - 002 runs the real production path (RunCycle) via the named core
//     integration test and asserts the orchestrator's probe decision agrees with
//     the predicate; the source assertion is auxiliary single-sourcing evidence.
//   - 003 runs the frozen pre-existing collision regression (no behaviour change)
//     and asserts the audit-binding Put site is routed through the predicate
//     rather than keeping its own copy of the comparison.
//   - 004 runs the verdictcache package suite and pins the ADR-0069 apicover
//     obligation for the new exported symbol.
```

### `go/acs/cycle1488/predicates_test.go:88` — above `func TestC1488_002_ShadowProbeWiredToSharedPredicate(t *testing.T) {`

```text
// TestC1488_002_ShadowProbeWiredToSharedPredicate is the wiring proof: the
// pre-loop ADR-0048 shadow probe must derive eligibility from the shared
// predicate, proven from the real RunCycle path by the differential oracle in
// internal/core/verdict_cache_probe_wiring_test.go.
```

### `go/acs/cycle1488/predicates_test.go:126` — above `if !acsassert.FileNotContains(t, bindings, "worktreeTree == headTree") {`

```text
// FileNotContains, not an inverted FileContains: the positive primitive
// Errorf's internally on the absent (correct) state, so the inverted idiom
// is red on every tree — the documented cycle-352 class, relived as the
// cycle-1488/1492/1495 three-burn anchor before this salvage repaired it.
```
