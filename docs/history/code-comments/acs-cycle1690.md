# Comment history: `acs/cycle1690`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1690/predicates_test.go:3` — above `package cycle1690`

```text
// Package cycle1690 materialises the cycle-1690 acceptance criteria for the one
// fleet-scoped inbox id `statejson-latent-unresolved-writers` (scout tasks
// `statemap-export-resolve-write-target` and `failurelog-symlink-resolve-writes`).
//
// The defect (latent, cycle-999 class). A worktree's .evolve/state.json is an
// ABSOLUTE symlink to the canonical host state file (core/worktree.go
// linkGuardDeps). statemap.WriteStateMap/UpdateStateMap already resolve that
// link before they lock and before they tmp+rename, so the link survives and
// cross-tree writers share ONE "<canonical>.lock" sidecar. Two writers do not:
//
//  1. core.SealCycle locks flock.WithPathLock(<evolveDir>/state.json) on the
//     UNRESOLVED path, so a seal through a linked evolve dir takes a sidecar no
//     canonical-path writer ever takes — the cross-tree lock unification does
//     not cover it (lost update).
//  2. every failurelog state writer (Record, PruneExpired,
//     PruneByClassification, PruneExpiredCarryoverTodos,
//     BackfillLegacyCarryoverExpiry, IncrementCarryoverUnpicked) tmp+renames
//     the raw path, which REPLACES the link with a regular file — the cycle-999
//     sever, after which every mutation strands in a detached copy.
//
// The accepted fix: export statemap.ResolveWriteTarget (same bounded,
// dangling-tolerant semantics as the unexported helper) and route both writers
// through it.
//
// Predicate strategy — every predicate exercises the system under test (the
// cycle-85 degenerate-predicate ban); every fixture lives under t.TempDir()
// (the worktree's own .evolve/state.json IS a live link to the host state
// file, so no predicate may ever touch a repo-relative .evolve path):
//
//   - 001 CALLS ResolveWriteTarget over every link shape and asserts the final
//     target, including the dangling tail and a bounded symlink loop; it also
//     proves the export agrees with the write target WriteStateMap really uses.
//   - 002 runs apicover's own AST + coverage detectors over the enrolled
//     statemap package (a new export no test names hard-fails
//     `make apicover-enforce` for the whole tree).
//   - 003/004/007 drive the PRODUCTION caller core.SealCycle: 003 observes which
//     "<path>.lock" sidecar it takes (the flock convention's own side effect),
//     004 proves it actually serializes with a canonical-path writer holding
//     that lock mid-RMW, 007 is the uncontended end-to-end cycle-999 shape.
//   - 005 drives all six failurelog writers through absolute, relative and
//     two-hop links (plus a regular-file baseline) and asserts every hop
//     survives and the canonical file carries the write.
//   - 006 is the negative edge: a dangling link must be left untouched by every
//     failurelog writer (Record keeps its ErrStateMissing contract — it never
//     auto-creates state.json).
//   - 008 proves the durable in-package regression tests the eval files name
//     actually RAN, passed, and are git-tracked (a `-run` pattern matching
//     nothing exits 0 — the vacuous-pass hole).
//   - 009 is the no-regression floor for the three touched packages.
//   - 010/011 are the audit-round-1 repair (M1). The lane item's how_to_apply
//     step (3) — the sweep of other atomicwrite users writing linkable .evolve/
//     state — was deferred in prose only, and a PASS landing consumes the whole
//     lane item (ship/postship.go committedInboxIDs, the cycle-1515
//     decomposition shape). 010 runs that consume through the ship's own
//     exported readers and resolver and requires a tracked follow-up record
//     that survives it; 011 requires the explanation Limitations to state the
//     consume and cite that record by an id the resolver finds.
```

### `go/acs/cycle1690/predicates_test.go:342` — above `func TestC1690_007_SealCycleEndToEndKeepsTheLinkAndLandsOnCanonical(t *testing.T) {`

```text
// TestC1690_007_SealCycleEndToEndKeepsTheLinkAndLandsOnCanonical is the
// uncontended cycle-999 shape through the production caller: SealCycle's
// failurelog.Record and its own RMW both write the state file, and today
// Record's raw tmp+rename replaces the worktree link with a regular file, so
// the seal's lastCycleNumber / failedApproaches strand in a detached copy.
// Both link encodings are covered (absolute is what linkGuardDeps creates).
```

### `go/acs/cycle1690/predicates_test.go:520` — above `const laneItemID = "statejson-latent-unresolved-writers"`

```text
// ---------------------------------------------------------------------------
// 010-011 — audit round 1 repair (M1): the deferred how_to_apply step (3)
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1690/predicates_test.go:657` — above `func passLandingConsumedIDs(t *testing.T) []string {`

```text
// passLandingConsumedIDs is the widest id set this cycle's PASS landing can
// consume, read through the same exported readers the ship closeout uses:
// triage's committed ids plus every non-deferred lane-scope id (top_n names no
// scope id — the cycle-1515 decomposition shape — so the whole scope rides the
// landing). A Closes-Inbox marker could only widen it; none may name step (3).
```

### `go/acs/cycle1690/predicates_test.go:1015` — above `func assertLinkIntact(t *testing.T, path, want string) {`

```text
// assertLinkIntact fails when path is no longer a symlink to want — the
// cycle-999 sever is exactly a rename replacing the link with a regular file.
```
