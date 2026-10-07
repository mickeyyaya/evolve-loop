# Build Explanation — Cycle 1821

## Build Binding
- Cycle: 1821
- Base SHA: 6292cf2bf35fe7fac01f83cd83185ff5c5de0911

## Summary
Every dispatch Plan built by `llmroute.Resolve` or `llmroute.ChainFor` now owns its trigger slice. Before this change a profile without `cli_fallback_on_exit` received the package-level `defaultFallbackOnExit` slice itself, so any caller that wrote to `Plan.Triggers` or appended through a reslice of it rewrote the default exit-code set for every later plan in the process. The same cycle drops the unused `lookPath` parameter of `ApplyUniversalFallback` and folds the near-duplicate `ApplyBench` / `ApplyDriverBench` into one implementation keyed by a bench-key function.

## Rationale
`resolveTriggers` already copied a profile's own list; only the default branch returned the shared slice. Returning `DefaultTriggers()`, the existing exported copy helper, makes both branches copy and reuses the one place that clones the default. The `lookPath` parameter had been discarded with `_ = lookPath` since presence stopped suppressing the universal tail, so every caller passed `nil`; removing it deletes a misleading seam. The two bench functions differed only in how a candidate maps to a bench key (`Family(cli)` versus `cli`), so a single `applyBenchKeyed` removes the duplicated partition and sort logic that could drift.

## Changed Areas
- `go/internal/llmroute/llmroute.go` — `resolveTriggers` returns `DefaultTriggers()` instead of the shared slice; `ApplyUniversalFallback` takes `(Plan, []string)`; `ApplyBench` and `ApplyDriverBench` delegate to `applyBenchKeyed` with `Family` and `driverName` keys.
- `go/internal/llmroute/trigger_aliasing_test.go` — new regression tests: editing (element write and append through a reslice) one plan's triggers never changes a later plan's or `DefaultTriggers()`, for nil, nil-list, empty-list and model-tier-only profiles, through both `Resolve` and `ChainFor`; a profile's own list wins and is never aliased.
- `go/internal/llmroute/universal_fallback_test.go` — calls updated to the two-argument signature; the now-unused `lookPathStub` helper and stale presence comments removed.
- `go/internal/cliroute/legacy.go` — production caller updated to the two-argument `ApplyUniversalFallback`.
- `go/internal/cliroute/table.go` — production caller updated to the two-argument `ApplyUniversalFallback`.
- `docs/architecture/packages/internal-llmroute.md` — records the "every Plan owns its Triggers" invariant, the single keyed bench implementation, and the dropped `lookPath` parameter.

## Design Decisions
The bench key is a `func(cli string) string` rather than a boolean or enum, so each exported entry point states its scope by naming its key function (`Family`, `driverName`) and the shared body has no branching on scope. Both exported entry points stay, because `bridgechain`, `usageprobe` and the runner call them by name. The trigger copy happens at resolve time, not at each consumer, so every current and future caller of `Resolve`/`ChainFor` is protected.

## Verification
`go test -count=1 -race ./internal/llmroute/ ./internal/cliroute/ ./internal/guards/ ./internal/bridgechain/ ./internal/usageprobe/ ./internal/phases/runner/` passes. The new `TestResolveTriggers_EveryPlanOwnsItsDefaultTriggers` was run against the pre-fix `resolveTriggers` and failed with the default rewritten to `[999 81 4242 124 127]`, then passed with the fix. The cycle's ACS predicates `TestC1821_001` through `TestC1821_007` all pass, including the production-router reachability predicate through `cliroute.Router.Resolve` for both the dispatch (Resolve) and advisor (ChainFor) launches.

## Compatibility
`ApplyUniversalFallback` loses its third parameter; the two in-repo production callers passed `nil` and are updated in this diff. Plan contents, bench demotion order and the universal tail are unchanged; only slice ownership differs.

## Limitations
Overlay `Family()` handling (prescription 4 of the inbox item) is not part of this change; it moved to `bridge-family-manifest-facade`. Plans built directly as struct literals by callers can still share slices they construct themselves.
