# Build Explanation — Cycle 2327

## Build Binding
- Cycle: 2327
- Base SHA: aee40d4d52eeb0506e2da3a38d28190947689d6b

## Summary
The dev worktree sweep now treats `.evolve/` and `.commit-gate/` as rebuildable at any depth. A tree that holds only pipeline output passes the leftovers gate and the sweep can archive it.

## Rationale
Plan decision D3 and component X3 name the rebuildable list as the fix. The scout found 140 dev trees that held only pipeline output and were kept. Two map entries are the smallest change, because the existing matcher already handles a directory pattern at any depth.

## Changed Areas
- `go/internal/gc/devleftovers.go` — adds `.evolve/` and `.commit-gate/` to `devRebuildableIgnored`, each with its reason.
- `docs/architecture/packages/internal-gc.md` — the allowlist bullet (line 80) names the two new entries. The nested repository guard is stated in the earlier bullet (line 79) and does not change.
- `docs/explain/builds/cycle-2327-01m76epm6wgmc6h6rebj9gdyp4.md` — this document.

## Design Decisions
The change adds data to the existing map and no new code path. An `.env` file and a nested repository still keep the tree, because neither one matches a pattern.

## Verification
The tests in `go/internal/gc/devleftovers_test.go` cover pipeline output at any depth, a commit gate only tree, an `.env` beside pipeline output and a nested repository. ACS predicates TestC2327_001 to TestC2327_005 pass, and `rebuildableIgnored` keeps 100% coverage.

## Compatibility
Earlier rebuildable entries and the nested repository guard do not change. The sweep now archives trees that it kept before.

## Limitations
A tree with real work inside `.evolve/` counts as rebuildable. The plan accepts this, because that directory holds pipeline output only.
