# Comment history: `internal/setup`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/setup/recommend_tier_top_test.go:3` — above `import "testing"`

```text
// recommend_tier_top_test.go — RED tests (cycle 517, task
// advisor-tier-vocab-add-top) for the SECOND half of "wire the 'top' model
// tier through advisor + policy rank". Cycle 516 already wired the advisor
// half (sanitizeAdvisorTier, phase_advisor_tier_test.go — pre-existing GREEN,
// verified this cycle) and policy.TierRank itself already classifies "top" as
// rank 4 (policy_test.go TestTierRank — pre-existing GREEN). The remaining
// gap: package setup's own CONSUMERS of that policy rank never learned about
// rank 4.
//
//   - tierFromRank (recommend.go) only maps ranks 1-3 back to a tier string;
//     rank 4 ("top") falls through to "", so canonTier("top") == "" — the
//     setup/recommend flow cannot even round-trip the literal string "top".
//   - biasTier's "up" strategy hard-caps at `if r < 3 { r++ }`, so a
//     max-quality preset can never recommend "top" even when a phase's
//     envelope explicitly allows it.
//   - clampTier's floor-clamp (`rWant < rMin`) calls tierFromRank(rMin) to
//     produce the clamped value; when rMin is 4 ("top"), the same gap
//     silently downgrades a valid clamp target to the empty string — a
//     broken tier is worse than no clamp.
//   - abstractTiers (setup.go) is still the pre-4-tier {fast,balanced,deep}
//     literal, so tierModelsFor never surfaces a "top" key in CLIStatus.
//     TierModels at all (onboarding can never document/report it).
//
// RED today: every test below fails against the current 3-tier-only
// implementation. Do NOT modify this file — implement the seam.
//
// AMENDED 2026-07-27 (operator sign-off, model-tier-safety) — the ONE exception
// to "Do NOT modify" above: TestTierModelsFor_IncludesTopIdentityFallback pinned
// claude top→"top", now known to be a live fatal launch. Renamed to
// TestTierModelsFor_TopResolvesToModelNotTierName; see its doc for the contract
// (both halves preserved).
```

### `go/internal/setup/recommend_tier_top_test.go:92` — above `func TestTierModelsFor_TopResolvesToModelNotTierName(t *testing.T) {`

```text
// TestTierModelsFor_TopResolvesToModelNotTierName (AC, positive + negative):
// tierModelsFor must resolve a "top" entry for every CLI — abstractTiers must
// include "top" or onboarding can never document/report it. Both halves of
// that contract are pinned:
//
//   - a family whose manifest declares the tier resolves to a real MODEL id;
//   - the identity fallback survives for a base with NO manifest, which is the
//     only case where echoing the tier name is a feature rather than a latent
//     `--model <tier>` fatal launch (bridge.isUnresolvedModelToken is the sink
//     guard that makes even that case non-fatal).
//
// Hermetic without a catalog seam: the test binary's cwd is this package dir,
// which contains no .evolve, so paths.ResolveFromEnv finds no catalog to
// overlay. (The former t.Setenv("EVOLVE_MODEL_CATALOG_DIR") here was inert —
// that env read was replaced by fn-var DI in cycle-17 and acs/cycle17 asserts
// it has no reader; the real seam is bridge.SetModelCatalogDirFn.)
```

### `go/internal/setup/setup_test.go:108` — above `"fast":     "Gemini 3.7 Flash (Low)",`

```text
// Corrected 2026-08-28: these pinned "Gemini Flash 3.7 (...)", a
// transposition agy REJECTS (it warns once and serves Gemini 3.5
// Flash (Medium) for the session). The pin fossilized the defect —
// it was green the whole time the tier was silently downgraded.
```
