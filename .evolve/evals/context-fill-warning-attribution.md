---
score_cap:
  - criterion: "A contributor breakdown attached to a fill WARN carries the exact basis the caller supplies, alongside the phase name, and stays silent below/at threshold or on the unmeasured sentinel"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestFillWarnWithContributors_IncludesGivenBasis|TestFillWarnWithContributors_SilentBelowThresholdOrSentinel' ./internal/tokenusage"
  - criterion: "The contributor breakdown wired into the real dispatch path (Engine.recordTokenUsage) uses the PEAK single-turn components — the same basis FillPct itself is derived from — never the whole-launch summed total"
    max_if_missing: 9
    evidence: "cd go && go test -run TestContextFillWarn_ContributorsMatchPeakPromptReading ./internal/bridge"
  - criterion: "Tiers with no per-turn breakdown (events/scrollback) fall back to the whole-launch total for their contributors, unchanged from before this fix"
    max_if_missing: 7
    evidence: "cd go && go test -run TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData ./internal/bridge"
  - criterion: "A genuine over-100% overrun stays unclamped and legible once a contributor breakdown is attached to its WARN"
    max_if_missing: 7
    evidence: "cd go && go test -run TestFillWarnWithContributors_OverHundredPercentStaysLegible ./internal/tokenusage"
  - criterion: "Pre-existing peak-vs-sum false-positive protection (cycle-1455) and strict-threshold/phase-naming semantics (cycle-1444) are unbroken by the attribution fix"
    max_if_missing: 8
    evidence: "cd go && go test -run 'TestFillPct_UsesTerminalTurnNotSumOfTurns|TestFillWarn_CorrectedFixtureDoesNotFalsePositive|TestContextFillWarn_EmittedAtDispatchNamingPhase|TestContextFillWarn_BoundaryAndSentinelStaySilent' ./internal/tokenusage ./internal/bridge"
---

# Eval: Context-fill warning attribution

> Pins the fix for the cycle-1458 audit M1 finding: if a fill WARN ever grows
> a contributor breakdown, that breakdown must be measured on the SAME basis
> as the percentage it annotates — the fullest single observed turn — never
> the whole-launch summed total. Before this cycle no contributor breakdown
> existed at all; `FillWarn` reported only a phase and a percentage. This eval
> enforces that the breakdown introduced in cycle-1482
> (`FillWarnWithContributors` + `Result.PeakUsage` + the `recordTokenUsage`
> wiring) never lets an operator see a percentage and a contributor total that
> disagree, while leaving every pre-existing peak-vs-sum and threshold
> guarantee (cycle-1444, cycle-1455) untouched. Source: scout-report.md
> cycle-1482 Task 2; `.evolve/runs/cycle-1458/audit-report.md`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unit-contract | breakdown carries the given basis; silent below/at threshold or sentinel | 7/10 | `go test -run 'TestFillWarnWithContributors_IncludesGivenBasis\|TestFillWarnWithContributors_SilentBelowThresholdOrSentinel'` |
| wiring-crux | production path uses the PEAK turn's own components, not the sum | 9/10 | `go test -run TestContextFillWarn_ContributorsMatchPeakPromptReading` |
| fallback | no-per-turn tiers keep using the whole-launch total | 7/10 | `go test -run TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData` |
| legibility | over-100% overruns stay unclamped with a breakdown attached | 7/10 | `go test -run TestFillWarnWithContributors_OverHundredPercentStaysLegible` |
| regression | cycle-1444/1455 false-positive and threshold guarantees unbroken | 8/10 | `go test -run 'TestFillPct_UsesTerminalTurnNotSumOfTurns\|TestFillWarn_CorrectedFixtureDoesNotFalsePositive\|TestContextFillWarn_EmittedAtDispatchNamingPhase\|TestContextFillWarn_BoundaryAndSentinelStaySilent'` |
