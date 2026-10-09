# Comment history: `internal/failurelog`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## phase 3d r1 (subagent, failurelog, acssuite)

### `go/internal/failurelog/apicover_named_test.go:3` — above `package failurelog`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const LegacyEffectiveTTL (prune.go) — asserted via PruneExpired's legacy
//     recordedAt fallback (an entry recordedAt+LegacyEffectiveTTL in the past
//     is pruned; one inside the window is kept).
//   - const MaxEntries (record.go) — asserted via Record's FIFO trim cap.
//   - type PruneResult (prune.go) — asserted by full-struct equality on the
//     value PruneExpired returns.
//   - type Recorded (record.go) — asserted by full-struct equality on the
//     value Record returns.
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
```

### `go/internal/failurelog/carryover_lifecycle_test.go:3` — above `import (`

```text
// carryover_lifecycle_test.go — behavior + apicover naming tests for the
// carryoverTodos lifecycle trio (cycle-536 ship): backfill legacy expiry →
// prune → staleness counter. CI's apicover -enforce flagged all three
// UNCOVERED (no test named them); these tests close the gap by pinning each
// documented contract, not by merely naming the identifiers.
```

### `go/internal/failurelog/classifications.go:210` — above `func ComputeExpiresAt(c Classification, now time.Time) string {`

```text
// ComputeExpiresAt returns the ISO-8601 timestamp (UTC, second
// precision) at which an entry of the given classification expires.
//
// Ports failure_compute_expires_at from failure-classifications.sh:174-196.
// The bash version had a v8.23.1 bug where jq fromdateiso8601 failed
// silently on unquoted ISO strings, producing epoch+1day expiry. Go's
// time.Time arithmetic is structurally immune to that class of bug.
//
// If `now` is the zero value, time.Now().UTC() is used.
```

### `go/internal/failurelog/mid_execution_fail_test.go:10` — above `func TestClassificationMidExecutionFail_IsOutsideTheTaxonomy(t *testing.T) {`

```text
// ADR-0103 unit 03b, F11 pinned at its source: the default class is OUTSIDE
// the taxonomy, so a supervisor-synthesised FailedRecord (and its P0 todo)
// ages out on the ONE-DAY legacy bucket. Admitting the class or lengthening
// the TTL is an operator decision — this test turns red the day it is taken.
```

### `go/internal/failurelog/prune_carryover.go:11` — above `func PruneExpiredCarryoverTodos(statePath string, now time.Time) (PruneResult, error) {`

```text
// PruneExpiredCarryoverTodos walks state.json:carryoverTodos and removes entries
// whose expiresAt is in the past — the structurally-parallel sibling of
// PruneExpired (failedApproaches), applied to the array that today has no removal
// path at all (65 entries / 26KB, cycles 366→506). Semantics mirror PruneExpired
// exactly (single-sourced intent via the shared isExpired oracle):
//
//   - entry.expiresAt in the past      → removed
//   - entry with NO expiresAt (legacy) → KEPT (age unknown; never delete)
//   - missing / carryoverTodos-less    → {0,0,0}, nil (safe no-op)
//
// statePath is typically <projectRoot>/.evolve/state.json. now is usually
// time.Now().UTC(); the zero value means "use real now".
```

### `go/internal/failurelog/prune_carryover_test.go:3` — above `import (`

```text
// prune_carryover_test.go — RED tests (cycle 507, task
// prune-stale-carryover-todos) for the PRUNE half of the carryoverTodos TTL
// contract. Mirrors the existing PruneExpired (failedApproaches) behavior,
// applied to the structurally-parallel state.json:carryoverTodos array which
// today has no removal path at all (65 entries / 26,601 bytes, cycles 366→506).
//
// Semantics mirror PruneExpired exactly (single-sourced intent):
//   - entry.expiresAt in the past           → removed
//   - entry with NO expiresAt (legacy)      → KEPT (age unknown; never delete)
//   - missing / carryoverTodos-less state   → {0,0,0}, nil (safe no-op)
//
// References PruneExpiredCarryoverTodos, which the Builder implements beside
// PruneExpired in this package (and wires into cmd_loop.go's AutoPrune block).
// RED now (undefined symbol → failurelog test package fails to compile). Do NOT
// modify this file — implement the production seam.
```

### `go/internal/failurelog/record.go:240` — above `var atomicWriteJSON = func(path string, state map[string]any) error {`

```text
// atomicWriteJSON serializes state and writes it atomically (mv-of-tmp,
// POSIX rename is atomic on the same filesystem; no partial file on
// failure). Delegates to the shared internal/atomicwrite implementation.
// The rename lands on statemap.ResolveWriteTarget(path): renaming over a
// worktree's state.json link would replace the link with a regular file and
// strand every later write in a detached copy (cycle-999).
//
// Exposed via this seam so tests can drive the write-error branch
// without contriving filesystem permissions.
```

### `go/internal/failurelog/symlink_state_test.go:11` — above `func TestStateWriters_PreserveSymlinkedStatePath(t *testing.T) {`

```text
// TestStateWriters_PreserveSymlinkedStatePath is the cycle-1690 pin: every
// failurelog state writer tmp+renames through atomicWriteJSON, and a rename
// over a worktree's state.json link REPLACES the link with a regular file
// (the cycle-999 sever). Each writer must keep the link and land its write on
// the canonical file.
```

### `go/internal/failurelog/vocabulary_test.go:8` — above `func TestVocabularyList_NamesEveryKnownClassificationOnce(t *testing.T) {`

```text
// VocabularyList is the ONE rendering of the failure-class vocabulary that the
// audit contract block (prompt) and the deliverables gate (correction) hand an
// agent — cycle 1684's invented class showed the auditor had never been told
// the set. Every known classification appears exactly once; unknown does not.
```
