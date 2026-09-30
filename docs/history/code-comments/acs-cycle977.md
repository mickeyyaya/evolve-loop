# Comment history: `acs/cycle977`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle977/predicates_test.go:3` — above `package cycle977`

```text
// Package cycle977 materializes the cycle-977 acceptance criteria for this
// fleet lane's single scoped id `wire-or-remove-model-tier-overrides-consumer`.
//
// Background. cycle-974 greened its C974_003 disjunction via the REMOVED branch
// (it deleted three below-floor inert keys). But the remaining, semantically
// meaningful overrides — e.g. scout.json `cycle_1_or_low_goal=deep` — are STILL
// inert: `ResolveModelTier` (go/internal/subagent/modeltier.go) never consults
// `model_tier_overrides`. The "no inert API" goal constraint is unfinished.
// Triage committed exactly one top_n task (T1 — WIRE, not remove: removal would
// break the archived cycle974/cycle339 predicate compile that field-accesses
// `.ModelTierOverrides`). Predicates are authored for T1 only.
//
// Builder contract (from scout T1): in `ResolveModelTier`, after computing the
// base tier, read the profile's `model_tier_overrides`, select the active
// situation from the real request signal (`cycle_1_or_low_goal` active when
// req.Cycle <= 1), clamp the override value to the profile's
// `model_tier_envelope` via the EXISTING `policy.TierRank` (no new rank table),
// and apply it as a FLOOR (max(base, clampedOverride)). Vocabulary stays
// abstract (fast/balanced/deep/top). Empty/nil override map ⇒ base unchanged.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/subagent is importable from
// go/acs, so the load-bearing predicates C977_001/002/003 EXERCISE the live
// `subagent.ResolveModelTier` (the SUT) — they inject profile bodies (and the
// real scout.json) and assert on the RETURNED tier. A magic-string source edit
// cannot satisfy them; the resolver must actually consume the override. No
// predicate's sole assertion is a source grep.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C977_002 "clamp above max" (override=top, envelope max=deep) must
//	           return deep, NOT top: a naive apply-without-clamp is rejected, and
//	           the current ignore-override state (returns balanced) red-fails it.
//	           C977_003 asserts Cycle=5 returns the base tier (override inactive)
//	           so a hardcoded "always deep" no-op cannot pass.
//	EDGE     → C977_002 tables the nil/absent-map edge (base unchanged) and the
//	           floor-no-lower edge (override below default does not demote).
//	SEMANTIC → C977_001 (consumer raises within envelope), C977_003 (composed
//	           real-config producer-keyed path), C977_004 (cycle-974 regression),
//	           and C977_005 (rank-table reuse + vet clean) are DISTINCT outcomes.
//
// RED before T1: the resolver ignores overrides, so C977_001 (want deep, got
// balanced), C977_002's raise/clamp rows (got balanced), and C977_003's Cycle=1
// row (got balanced) all fail; C977_005 fails because modeltier.go does not yet
// reference policy.TierRank. C977_004 is regression coverage — green now (via
// cycle-974's REMOVED branch) and MUST stay green after wiring.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	AC1 production consumer reads+applies the override → C977_001 (predicate)
//	AC3 clamp-to-max negative + nil/floor edges         → C977_002 (predicate)
//	AC4 composed-path proof on real scout.json          → C977_003 (predicate)
//	AC2 cycle-974 predicate regression stays green       → C977_004 (predicate)
//	AC5 reuse policy.TierRank (no dup table) + vet clean  → C977_005 (predicate)
```

### `go/acs/cycle977/predicates_test.go:222` — above `func TestC977_004_Cycle974RegressionStillGreen(t *testing.T) {`

```text
// TestC977_004_Cycle974RegressionStillGreen is regression coverage for AC2: the
// cycle-974 predicate that first flagged the inert map
// (TestC974_003_OverridesConsumedOrRemoved) must still pass after this cycle's
// WIRE. It runs the cycle-974 acs predicate via a `go test -tags acs`
// subprocess. Green now (cycle-974's REMOVED branch) and green after (the WIRED
// branch) — a regression guard, not a state flip.
```
