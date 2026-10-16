# Comment history: `internal/sessionreaper`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/sessionreaper/attribution_test.go:11` — above `func TestReapOrphans_AttributionDiscoverableAfterTombstone(t *testing.T) {`

```text
// TDD RED (cycle-806, task sweep-tombstone-attribution).
//
// End-to-end proof that a session killed by ReapOrphans stays attributable to
// its owning run AFTER the registry is tombstoned. This is the exact accounting
// hole the soak test's Invariant 3 hit: post-reap the live registry is renamed
// to `.reaped`, so sessionrecord.ReadAll(PathIn(runDir)) finds nothing. The
// tombstone-aware sessionrecord.ReadAllResolving must recover it. RED until
// Builder adds ReadAllResolving (compile failure — symbol absent).
```

### `go/internal/sessionreaper/sessionreaper.go:25` — above `const reapedSuffix = sessionrecord.ReapedSuffix`

```text
// reapedSuffix tombstones a fully-reaped registry. Renaming (rather than
// deleting) preserves the session audit trail, and the existing
// "no registry → skip" walk check makes every later sweep skip the run for
// free — the fix for the unbounded boot re-reap class (cycle-769: 4,388
// sessions re-swept serially on every boot). A relaunched run recreates the
// registry, so it becomes sweepable again. The suffix value lives in the
// sessionrecord SSOT (ReapedSuffix) so the reader that resolves the tombstone
// for attribution (sessionrecord.ReadAllResolving) shares one definition.
```

### `go/internal/sessionreaper/tombstone_test.go:12` — above `func makeRunIn(t *testing.T, evolveDir, name, session string) (string, string) {`

```text
// Cycle-769 boot-orphan-sweep-bounded-tombstone regression contract.
//
// Incident (inbox 2026-07-08, weight 0.90): every loop boot re-reaps the
// ENTIRE run history — 4,388 recorded sessions across 304 registries —
// because a fully-reaped registry is never marked done, so idempotence is
// achieved by redoing all work, forever, with ~7 sessions/cycle growth.
//
// Contract: a run whose registry was reaped FULLY SUCCESSFULLY is skipped by
// subsequent sweeps; any partial failure (killer error) leaves the run
// unmarked so the next sweep retries it. The marker mechanism (rename vs
// sibling file) is the implementer's choice — these tests observe only kill
// traffic across consecutive sweeps.
```
