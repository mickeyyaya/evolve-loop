# internal/contextfill

## Purpose

`internal/contextfill` derives how full the model context window of a phase got. It uses the token counts that the loop already persists. Every phase records a `cyclestate.TokenUsage` (input, output, cache read, cache write) into `<workspace>/phase-timing.json`. This package turns those absolute counts into one number: occupancy over the window size.

## Design

- **Pure derivation.** The package is a leaf over the stdlib and `cyclestate`. It is wired into the durable telemetry record and nowhere else. `recordPhaseOutcome` in `internal/core` (the ADR-0044 C1 chokepoint) derives the ratio onto `phasetiming.Entry`. `phasetiming.Rollup` summarizes the hot phases through `IsHot`. Those two packages are the whole importer set.
- **Deferred.** The off, advisory and enforce stage dial, and the advisory prompt hint, are deferred to later work.
- **`FillRatio`.** Every `TokenUsage` field counts: uncached input, cache reads, cache writes and generated output all use window space.
- **`WindowSizeForTier`.** It is a flat per-tier stub, not a per-model registry. Every Claude tier that the loop routes to shares one 200k-token window. When that stops holding, key a registry off the resolved model id, as the tables of `internal/modelcatalog` do. Keep this function as the tier-level fallback.

## Invariants

- **The ratio is not clamped at 1.0.** A phase that overran its window by half stays distinguishable from one that just fit. That is the diagnostic point.
- **A non-positive window is an error.** `FillRatio` returns `(0, ErrInvalidWindow)`. An unknown window then surfaces as unknown. It never becomes a made-up fill number or a divide-by-zero NaN or Inf.
- **An unknown tier has no default.** `WindowSizeForTier` returns 0 for an unknown or empty tier, and `FillRatio` rejects it. No window is invented for a tier that cannot be identified.
- **`HotThreshold` is inclusive.** A phase at 85 percent or more of its window is hot, and its output is at risk of truncation. A ratio exactly at the threshold is hot (`IsHot`).
- **Keep the importer set.** Only `internal/core` and `phasetiming` import this package.
