# Comment history: `acs/cycle517`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle517/predicates_test.go:3` — above `package cycle517`

```text
// Package cycle517 materialises the cycle-517 acceptance criteria.
//
// TRIAGE COMMITTED EXACTLY ONE ## top_n TASK this cycle (triage-decision.json /
// triage-report.md — this is a fleet lane; the scout-report.md's two
// wave/boot-recovery findings were both DEFERRED — "not in the assigned fleet
// scope for this cycle's concurrent execution lane" — so, per R9.3, no
// predicates are authored for them here):
//
//	advisor-tier-vocab-add-top (carryover, priority=H, evidence=
//	go/internal/core/phase_advisor_tier_test.go:2) — "Wire the 'top' model tier
//	through advisor + policy rank". Investigation this cycle found the ADVISOR
//	half already landed in cycle 516 (sanitizeAdvisorTier accepts "top";
//	policy.TierRank classifies "top" as rank 4 — both pre-existing GREEN,
//	verified below as regression pins). The REMAINING gap is entirely within
//	go/internal/setup: package setup's own CONSUMERS of policy.TierRank's rank
//	4 were never updated —
//	  - tierFromRank (recommend.go) only maps ranks 1-3 back to a tier string,
//	    so canonTier("top") == "" (the setup/recommend flow cannot round-trip
//	    the literal string "top" at all).
//	  - biasTier's "up" strategy hard-caps at `if r < 3 { r++ }`, so it can
//	    never climb to "top" even when the envelope allows it.
//	  - abstractTiers (setup.go) is still the pre-4-tier {fast,balanced,deep}
//	    literal, so tierModelsFor never surfaces a "top" key at all.
//	The builtin "max-quality" preset (tier_bias="max") is broken end-to-end by
//	the first gap: biasTier's "max" branch calls canonTier(env.Max), which
//	returns "" for "top", silently falling back to the phase's base tier
//	instead of recommending "top".
//
// AC map:
//
//	AC-1 sanitizeAdvisorTier accepts "top" (advisor half)       -> C517_001 (behavioral; pre-existing GREEN, cycle-516 landed)
//	AC-2 policy.TierRank classifies "top" as rank 4              -> C517_002 (behavioral; pre-existing GREEN, cycle-516 landed)
//	AC-3 canonTier round-trips "top" (tierFromRank rank-4 gap)   -> C517_003 (behavioral, RED)
//	AC-4 "up" bias strategy can reach "top"                      -> C517_004 (behavioral, RED)
//	AC-5 clamping UP to a "top" floor does not degenerate to ""  -> C517_005 (behavioral, negative, RED)
//	AC-6 max-quality preset end-to-end recommends "top"           -> C517_006 (behavioral, RED)
//	AC-7 tierModelsFor surfaces a "top" key (identity fallback)  -> C517_007 (behavioral, RED)
//	AC-8 go vet ./go/..., existing setup/policy/core suites stay -> manual+checklist (Auditor):
//	     green, apicover -enforce clean on touched packages          run `go vet ./go/...`;
//	     (go/internal/setup)                                         `go test ./go/internal/setup/... ./go/internal/policy/... ./go/internal/core/...`;
//	                                                                  `apicover -enforce` scoped to go/internal/setup
//
// 1:1 enforcement: 8 total ACs = 7 predicate + 1 manual+checklist + 0
// unverifiable-remove.
//
// Predicate strategy (mirrors cycle503/cycle507/cycle514): BEHAVIORAL
// predicates drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan
// closes the cycle-85 "no tests to run" trap) — never a source grep. The
// in-package tests were authored by the TDD engineer:
//
//	internal/core/phase_advisor_tier_test.go   (AC-1, pre-existing from cycle 516)
//	internal/policy/policy_test.go             (AC-2, pre-existing from cycle 516)
//	internal/setup/recommend_tier_top_test.go  (AC-3..AC-7, new this cycle)
//
// The Builder implements production code ONLY (tierFromRank + biasTier's "up"
// numeric cap in recommend.go; abstractTiers in setup.go); it must not modify
// the tests.
```

### `go/acs/cycle517/predicates_test.go:97` — above `func TestC517_001_AdvisorSanitizerAcceptsTop(t *testing.T) {`

```text
// TestC517_001_AdvisorSanitizerAcceptsTop (AC-1, positive, pre-existing
// GREEN): sanitizeAdvisorTier must pass "top" through unchanged (cycle-516
// landed this). Pinned as a regression guard so this cycle's setup-package
// fix cannot silently coincide with an advisor-side regression.
```

### `go/acs/cycle517/predicates_test.go:109` — above `func TestC517_002_PolicyTierRankClassifiesTop(t *testing.T) {`

```text
// TestC517_002_PolicyTierRankClassifiesTop (AC-2, positive, pre-existing
// GREEN): policy.TierRank must classify "top" as rank 4 (cycle-516 landed
// this). Pinned as a regression guard — every fix in this cycle builds
// directly on this rank.
```

### `go/acs/cycle517/predicates_test.go:168` — above `func TestC517_007_TierModelsForSurfacesTop(t *testing.T) {`

```text
// TestC517_007_TierModelsForSurfacesTop (AC-7, positive, RED): tierModelsFor
// must surface a "top" key (identity fallback) for every CLI so onboarding
// can document/report it. Drives internal/setup
// TestTierModelsFor_TopResolvesToModelNotTierName (renamed 2026-07-27 with
// operator sign-off: claude-tmux.json now declares "top", so pinning the
// literal tier name as claude's model would pin a fatal-launch defect; the
// identity fallback stays pinned there on a manifest-less CLI).
```
