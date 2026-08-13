---
score_cap:
  - criterion: "Context-fill% is derived from ONE turn's prompt-side tokens, never the sum across a phase's assistant turns"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run TestFillPct_UsesTerminalTurnNotSumOfTurns ./internal/tokenusage"
  - criterion: "Result.Usage remains the SUM across turns — the cost/spend figure must survive the fill% fix untouched"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestScanConfigRoot_MultiTurnTranscript_UsageStaysTheSum ./internal/tokenusage"
  - criterion: "An honest single-turn overrun stays unclamped and legible above 100%, and still fires the fill WARN"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestFillWarn_OverHundredPercent_StaysLegible ./internal/tokenusage"
  - criterion: "A launch whose real reading is below the warn threshold does not warn because the summed reading crossed it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestFillWarn_CorrectedFixtureDoesNotFalsePositive ./internal/tokenusage"
  - criterion: "Zero observed in-window turns degrade to FillPctUnmeasured, never to a measured 0%"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestFillPct_ZeroObservedTurns_IsUnmeasured ./internal/tokenusage"
---

# Eval: context-fill ratio reported above 100%

> Pins the numerator contract of the `tokenusage` fill-percent path. `ScanConfigRoot`
> sums every assistant turn's `Input + CacheRead + CacheWrite` into one grand total
> (`scanner.go:147-153`), and `DefaultResolver` feeds that summed total into `FillPct`
> against a **single-turn** 200K effective window (`defaultresolver.go:38`). Each turn's
> own `cache_read_input_tokens` already represents that turn's entire prior context, so
> summing turn N with turn N+1 re-counts the same context once per turn: an N-turn phase
> near the ceiling reports roughly N × 100%. Source incident: cycle-1455, from the inbox
> item `contextfill-ratio-over-100pct` (2026-08-12) — two readings in one monitored wave,
> scout **566.9%** and triage **114.3%**.
>
> The eval deliberately guards BOTH directions. Three of the five caps exist so the fix
> cannot be a clamp: `Result.Usage` must stay summed (it is the cost number, and correct),
> a genuine overrun must stay legible above 100% (`fillpct.go` promises over-full readings
> are never clamped — "a launch at 120% is a real signal"), and a zero-observation launch
> must degrade to the negative sentinel rather than to a fabricated 0%. A fix that greens
> only the first cap by clamping, or by making the scanner stop summing, reds the rest.
>
> Note: the fill-percent path in `internal/contextfill` (`FillRatio`, consumed by
> `core.recordPhaseOutcome`) is a **different** module and does not carry this defect —
> it reads the terminal attempt's `out.Tokens`, not a per-turn transcript scan. This eval
> scopes to `internal/tokenusage` only.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| numerator-is-one-turn | Fill% derives from one turn, not the sum across turns | 3/10 | `go test -run TestFillPct_UsesTerminalTurnNotSumOfTurns ./internal/tokenusage` |
| cost-total-preserved | `Result.Usage` stays the summed cost figure | 4/10 | `go test -run TestScanConfigRoot_MultiTurnTranscript_UsageStaysTheSum ./internal/tokenusage` |
| over-100-still-legible | Honest overrun unclamped at 120% and still warns | 6/10 | `go test -run TestFillWarn_OverHundredPercent_StaysLegible ./internal/tokenusage` |
| no-false-positive-warn | 47%-real launch does not warn off a 99% summed artefact | 6/10 | `go test -run TestFillWarn_CorrectedFixtureDoesNotFalsePositive ./internal/tokenusage` |
| sentinel-preserved | Zero observed turns ⇒ `FillPctUnmeasured`, never 0% | 7/10 | `go test -run TestFillPct_ZeroObservedTurns_IsUnmeasured ./internal/tokenusage` |
