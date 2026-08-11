---
score_cap:
  - criterion: "sanitizeAdvisorTier accepts the 'top' tier (advisor half, cycle-516 regression guard)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestSanitizeAdvisorTier ./internal/core/"
  - criterion: "policy.TierRank classifies 'top' as rank 4 (policy-rank half, cycle-516 regression guard)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestTierRank ./internal/policy/"
  - criterion: "canonTier round-trips the literal 'top' (tierFromRank maps rank 4 back)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestCanonTier_TopPassesThrough ./internal/setup/"
  - criterion: "the generic 'up' bias strategy can climb to 'top' when the envelope allows it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows ./internal/setup/"
  - criterion: "clamping UP to a 'top' envelope floor does not degenerate to the empty string"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty ./internal/setup/"
  - criterion: "the shipped max-quality preset recommends 'top' end-to-end via Recommend()"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestRecommend_MaxQualityBiasesToTop ./internal/setup/"
  - criterion: "tierModelsFor surfaces a 'top' key (identity fallback) for onboarding"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestTierModelsFor_IncludesTopIdentityFallback ./internal/setup/"
  - criterion: "existing setup/policy/core regression suites and go vet stay green"
    max_if_missing: 5
    evidence: "cd go && go vet ./internal/setup/... ./internal/policy/... ./internal/core/... && go test -count=1 ./internal/setup/... ./internal/policy/... ./internal/core/..."
---

# Eval: Wire the "top" model tier through setup's policy-rank consumers

> Pins the remaining half of the cycle-516 carryover task
> `advisor-tier-vocab-add-top` ("Wire the 'top' model tier through advisor +
> policy rank"). Investigation in cycle 517 found the ADVISOR half already
> landed in cycle 516 (`sanitizeAdvisorTier` accepts `"top"`; `policy.TierRank`
> classifies `"top"` as rank 4) — both verified GREEN and pinned here as
> regression guards. The actual remaining gap is entirely within
> `go/internal/setup`: its own consumers of `policy.TierRank`'s rank 4 were
> never updated. `tierFromRank` (recommend.go) only maps ranks 1-3 back to a
> tier string, so `canonTier("top") == ""` — the setup/recommend flow cannot
> round-trip the literal string "top" at all. `biasTier`'s "up" strategy
> hard-caps its numeric increment at rank 3, so it can never climb to "top"
> even when an envelope allows it. `abstractTiers` (setup.go) is still the
> pre-4-tier `{fast,balanced,deep}` literal, so `tierModelsFor` never surfaces
> a "top" key at all. Combined, these three gaps mean the shipped
> "max-quality" preset (`tier_bias="max"`) silently falls back to a phase's
> base tier instead of ever recommending "top", even when a phase's envelope
> explicitly allows it — the onboarding flow's own policy-rank wiring was
> never extended past 3 tiers. Source incident: cycle 517 (this cycle),
> triage-committed as the sole `top_n` task for this fleet lane.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| advisor-regression | sanitizeAdvisorTier still accepts "top" | 4/10 | `go test -run TestSanitizeAdvisorTier` |
| policy-rank-regression | policy.TierRank still classifies "top" as rank 4 | 4/10 | `go test -run TestTierRank` |
| canon-round-trip | canonTier("top") == "top" | 8/10 | `go test -run TestCanonTier_TopPassesThrough` |
| up-bias-reaches-top | generic "up" bias strategy reaches "top" | 6/10 | `go test -run TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows` |
| clamp-no-degenerate | clamping up to a "top" floor never yields "" | 7/10 | `go test -run TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty` |
| max-quality-e2e | shipped max-quality preset recommends "top" | 8/10 | `go test -run TestRecommend_MaxQualityBiasesToTop` |
| tiermodels-surfaces-top | tierModelsFor includes a "top" key | 6/10 | `go test -run TestTierModelsFor_IncludesTopIdentityFallback` |
| suite-green | setup/policy/core suites + go vet stay green | 5/10 | `go vet` + full suite run |
