# Comment history: `acs/cycle872`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle872/predicates_test.go:3` — above `package cycle872`

```text
// Package cycle872 materializes the cycle-872 acceptance criteria for this
// fleet lane's sole committed task, wire-tier-fallback-chain (fleet_scope
// todo-id: overlay-injection-dormant-wire-fable-deep).
//
// Scout confirmed the operator-authored `tier_fallbacks` key in
// .evolve/model-catalog.json is dead config: encoding/json silently drops it
// because modelcatalog.CLIEntry declares no matching field, and
// DispatchModel/Lookup are single-shot with no chain traversal. The task adds
// `TierFallbacks map[string][]string` to CLIEntry and makes DispatchModel and
// Lookup walk the tier's chain when the primary TierModels entry is empty,
// returning the first non-empty model. Behavior with no fallbacks configured
// must be byte-identical to today (manifest fallback preserved).
//
// Every predicate below exercises the system under test directly: it imports
// the modelcatalog package, constructs a Catalog by unmarshaling JSON (the
// exact ingestion path .evolve/model-catalog.json takes), and asserts on
// DispatchModel/Lookup return values — no source-grep predicates.
```
