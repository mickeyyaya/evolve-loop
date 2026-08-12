---
score_cap:
  - criterion: "Context fill% is a derived reading (prompt-side tokens / per-family effective window) expressed 0-100, with output tokens excluded"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestFillTelemetry_PctFromPromptTokensAndWindow|TestFillTelemetry_PromptTokensSumsInputSideOnly' ./internal/tokenusage"
  - criterion: "An unconfigured/zero effective window yields an explicit negative unmeasured sentinel — never Inf, NaN, or a false 0%"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestFillTelemetry_ZeroWindowGuard|TestFillTelemetry_EffectiveWindowUnconfiguredFamily' ./internal/tokenusage"
  - criterion: "Fill% is single-sourced off the production resolver's already-recovered usage, and an uncovered launch carries the sentinel rather than reading as 0% full"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestFillTelemetry_ResolverStampsFillPct|TestFillTelemetry_UnmeasuredResolveCarriesSentinel' ./internal/tokenusage"
---

# Eval: Per-launch context-fill telemetry (FillPct)

> Pins the measurement contract of the context-fill instrument introduced in
> cycle 1444 for inbox item `context-fill-telemetry-and-cap`: fill% is a
> *derived* reading computed off the usage `tokenusage.DefaultResolver` already
> recovers — never a second independent scan — and an unmeasurable reading is an
> explicit negative sentinel rather than a divide-by-zero or a plausible-looking
> 0%. The sentinel is the load-bearing half: the 2026-07-13 all-zeros baseline
> defect (which motivated the existing per-driver coverage `Warn`) was exactly a
> case of unmeasured launches masquerading as measured-cheap ones, and a fill
> instrument that stamps 0.0 on every uncovered launch would reintroduce that
> class one level up — with the added cost that the deferred fill%-vs-verdict
> correlation report would then be regressing against fabricated zeros.
> Source incident: cycle 1444 (new instrument; no prior fill concept existed).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| derived-percentage | prompt-side ÷ effective window, 0–100 scale, output excluded | 6/10 | `go test -run 'TestFillTelemetry_PctFromPromptTokensAndWindow\|TestFillTelemetry_PromptTokensSumsInputSideOnly'` |
| unmeasured-sentinel | zero/unknown window ⇒ negative sentinel, never Inf/NaN/0 | 8/10 | `go test -run 'TestFillTelemetry_ZeroWindowGuard\|TestFillTelemetry_EffectiveWindowUnconfiguredFamily'` |
| single-sourced-resolve | stamped by the production resolver off its own recovered usage | 7/10 | `go test -run 'TestFillTelemetry_ResolverStampsFillPct\|TestFillTelemetry_UnmeasuredResolveCarriesSentinel'` |
