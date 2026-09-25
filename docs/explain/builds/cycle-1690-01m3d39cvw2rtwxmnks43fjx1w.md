# Build Explanation — Cycle 1690

## Build Binding
- Cycle: 1690
- Base SHA: 3c0962620bbf419f5b951e7b7f8bbcc68a68a0c6

## Summary
Two writers of the shared `state.json` did not resolve a worktree's symlink to the canonical state file. `core.SealCycle` took its state lock beside the link, so it did not exclude writers that lock the canonical path. Every `failurelog` state writer ran tmp+rename on the raw path, which replaced the link with a regular file (the cycle-999 sever). The build exports the existing chain walk as `statemap.ResolveWriteTarget` and sends both writers through it.

## Rationale
`statemap.WriteStateMap` and `UpdateStateMap` already resolve the link before locking and before renaming. The other writers need to agree with them on the target path and on the `<target>.lock` sidecar. The bounded, dangling-tolerant helper already existed and was tested, so the build exports it by renaming it rather than copying it. That gives every writer one shared definition of the write target.

## Changed Areas
- `go/internal/adapters/statemap/statemap.go` — renames `resolveWriteTarget` to the exported `ResolveWriteTarget`, adds godoc covering the lock-the-resolved-path contract, and updates the two internal callers. The logic is unchanged.
- `go/internal/adapters/statemap/statemap_integrity_test.go` — adds `TestResolveWriteTarget`. It covers regular, missing, absolute, relative, two-hop, dangling and loop inputs, so the export is named and executed (apicover enrollment).
- `go/internal/core/reset.go` — `SealCycle` now takes `flock.WithPathLock` on `statemap.ResolveWriteTarget(statePath)`, so a seal serializes with canonical-path writers across trees.
- `go/internal/core/reset_symlink_test.go` — adds `TestSealCycle_SymlinkedStateLocksCanonicalTarget`. It checks that the canonical sidecar is locked, that no sidecar is created beside the link, that the link survives, and that the write lands on the canonical file.
- `go/internal/failurelog/record.go` — the `atomicWriteJSON` seam renames onto `statemap.ResolveWriteTarget(path)`. This one seam covers all six write sites in record.go, prune.go and prune_carryover.go.
- `go/internal/failurelog/symlink_state_test.go` — adds `TestStateWriters_PreserveSymlinkedStatePath`. It runs all six writers through absolute and relative links.
- `go/acs/cycle1690/predicates_test.go` — the TDD-authored cycle predicates, committed unmodified.
- `.evolve/evals/statejson-latent-unresolved-writers.md` — the TDD-authored authority eval for the lane item, committed unmodified.
- `.evolve/evals/statemap-export-resolve-write-target.md` — the TDD-authored eval for top_n task 1, committed unmodified.
- `.evolve/evals/failurelog-symlink-resolve-writes.md` — the TDD-authored eval for top_n task 2, committed unmodified.
- `.evolve/inbox/2026-09-26T00-00-00Z-atomicwrite-linked-state-sweep.json` — a new inbox record that carries the deferred how_to_apply step (3) forward, because a PASS landing consumes the lane item (see Limitations).

## Design Decisions
- The fix resolves inside the `atomicWriteJSON` seam instead of at each of the six call sites. Changing one line cannot drift, and a future writer added to the package inherits the resolve step.
- The failurelog writers resolve but do not lock. `SealCycle` already holds the state flock across `failurelog.Record`, and flock is per open file description, so an internal lock in Record would self-deadlock.
- Record's `ErrStateMissing` contract is unchanged. Its existence check follows the link, so a dangling link still reports missing and nothing is created.

## Verification
- The new durable tests failed before the fix: 12/12 failurelog link rows, plus the core seal pin with canonical-lock-absent, link-lock-present and link-severed. They pass after the fix.
- The cycle-1690 ACS predicates pass 11/11. 010 and 011 check that the step-(3) follow-up survives this cycle's consume and that this section cites it.
- The full `go test -count=1 ./...` run exits 0. `go vet ./...` and `gofmt -l .` are clean.

## Compatibility
No behavior changes for regular-file state paths, because the resolve step is the identity for a non-link. The only API change is the new export. Nothing outside the package called the unexported name.

## Limitations
This build does not cover the inbox item's third how_to_apply step, a sweep of other `atomicwrite` users that target `.evolve/` state files. A PASS landing consumes the whole lane item `statejson-latent-unresolved-writers`: triage's `top_n` names only the two decomposed ids, so the ship closeout's `committedInboxIDs` adds the entire lane scope to the consumed set. Step (3) would close with the parent, so this build files it as its own inbox record, `atomicwrite-linked-state-sweep` (`.evolve/inbox/2026-09-26T00-00-00Z-atomicwrite-linked-state-sweep.json`). That record has a distinct id, names the parent as its lineage, carries its own acceptance criteria, and is not consumed by this cycle's landing.
