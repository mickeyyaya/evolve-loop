# Build Explanation — Cycle 2091

## Build Binding
- Cycle: 2091
- Base SHA: 36e174fb639c8da0b8631af65621b5f4a3e0c6b3

## Summary
An overlay rule can now select by the paths of the lane scope. A rule with no paths still matches every scope.

## Rationale
The docs-write skill reached every code-writing dispatch. A paths selector lets a rule load only when the scope has a matching file. The selector joins the existing intersection of selectors, so no new mechanism exists.

## Changed Areas
- `go/internal/policy/overlays.go` — adds `OverlayRule.Paths`, `OverlayDispatch.ScopePaths`, `matchPaths`, `matchPathSegments` and `validatePaths`.
- `go/internal/policy/patch.go` — `Parse` calls `validatePaths`, so a malformed glob fails at load.
- `go/internal/policy/overlays_paths_test.go` — tests the glob semantics and the load refusal.
- `docs/architecture/packages/internal-policy.md` — describes the selector and the glob rules.
- `docs/architecture/guidance-layers.md` — names the dispatch field in the path trigger row.

## Design Decisions
Globs split on `/`. The `**` segment matches zero or more segments, and `path.Match` handles the rest. This keeps `*` inside one segment and needs no new dependency.

## Verification
The cycle predicates and the policy package tests pass. The three path functions have 100 percent coverage.

## Compatibility
The compiled default rules are unchanged. An omitted `paths` key keeps the old behavior.

## Limitations
The runner does not fill `ScopePaths` yet, and the default docs rule is not split. Both are console work on a protected surface.
