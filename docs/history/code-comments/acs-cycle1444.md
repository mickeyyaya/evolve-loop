# Comment history: `acs/cycle1444`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1444/predicates_test.go:3` — above `package cycle1444`

```text
// Package cycle1444 materialises the cycle-1444 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item context-fill-telemetry-and-cap):
//
//   - context-fill-telemetry-record  → per-launch prompt-fill telemetry, derived
//     from the usage the existing resolver already recovers, with an explicit
//     unmeasured sentinel instead of a divide-by-zero or a false 0%.
//   - context-fill-warn-threshold    → a policy-configured WARN past that fill,
//     naming the phase, emitted from the production dispatch seam and persisted
//     into the launch record.
//
// Predicate strategy — every predicate exercises the system, never greps source
// (the cycle-85 degenerate-predicate ban):
//
//   - 001–003 call the real tokenusage API and drive the real production
//     resolver (DefaultResolver) over an on-disk events fixture.
//   - 004 calls the real policy resolver across the absent/empty/override/
//     out-of-range matrix.
//   - 005–006 are the REACHABILITY predicates: they shell one narrowed `go test`
//     each at the two production callers (the engine dispatch seam and the
//     adapter composition root), because those seams are unexported and a
//     predicate that called the helper directly would pass on dead code.
//     Each is ONE named package narrowed with -run (never a ./... sweep), per
//     the flaky-predicate-shape rules.
```

### `go/acs/cycle1444/predicates_test.go:40` — above `const claudeWindow = 200_000`

```text
// claudeWindow is the conservative effective window for the claude family
// (200K, per the 2026-08-03 reliability finding in the inbox item).
```
