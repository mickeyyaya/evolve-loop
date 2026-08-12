---
score_cap:
  - criterion: "context_fill.warn_threshold_pct resolves absent/empty/out-of-range to the built-in 60 and respects a valid 1-100 override"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestContextFillConfig ./internal/policy"
  - criterion: "The fill WARN is emitted from the production dispatch seam (Engine.recordTokenUsage), names the phase, fires only strictly above threshold, and stays silent on the unmeasured sentinel"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestContextFillWarn ./internal/bridge"
  - criterion: "The operator's policy.json threshold reaches the production engine deps through the composition root (config is not dead)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestProductionDeps.*ContextFill ./internal/adapters/bridge"
  - criterion: "A wrapped or negative prompt-side token total degrades to the FillPctUnmeasured sentinel — never to a fabricated negative percentage that is persisted to llm-calls.ndjson and silently suppresses the WARN"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestFillTelemetry ./internal/tokenusage"
---

# Eval: Context-fill WARN threshold (policy-configured, wired at dispatch)

> Pins the alerting half of cycle 1444's context-fill work: a policy-configured
> threshold (default 60% of the effective window) past which a launch's fill
> reading raises a phase-named WARN from the one production site every Launch's
> token telemetry funnels through. Two failure modes are guarded explicitly.
> First, operator input is never accepted verbatim — absent, empty, and
> out-of-range all resolve to the visible built-in, so a typo can neither silence
> the instrument nor arm it on every launch (the resolution shape is
> `ParallelEvaluatePolicy`'s, deliberately, so this block cannot drift into a
> novel config idiom). Second, and the reason two of the three caps are
> reachability rather than unit checks: a threshold that resolves correctly but
> never travels from `.evolve/policy.json` through the composition root into
> `Deps` is dead config, and a WARN helper whose only caller is a test is dead
> code. Both halves are pinned at their real callers.
> Source incident: cycle 1444 (new instrument).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| threshold-resolution | absent/empty/out-of-range ⇒ 60; valid override respected | 7/10 | `go test -run TestContextFillConfig ./internal/policy` |
| warn-at-dispatch-seam | phase-named WARN, strictly-above boundary, sentinel silent, persisted | 8/10 | `go test -run TestContextFillWarn ./internal/bridge` |
| policy-reaches-deps | composition root carries the resolved threshold into engine Deps | 8/10 | `go test -run TestProductionDeps.*ContextFill ./internal/adapters/bridge` |
| overflow-degrades-to-sentinel | wrapped/negative prompt total ⇒ sentinel, not a fabricated percentage | 7/10 | `go test -run TestFillTelemetry ./internal/tokenusage` |

> Cycle-1446 addendum: the overflow cap closes finding M1 of cycle 1444's WARN
> verdict — `PromptTokens` summed three driver-controlled counters with no
> saturation, so a wrapped sum reached `FillPct` and published a negative that
> was neither a real reading nor the documented sentinel, while `FillWarn`'s
> "any negative is unmeasured" rule silently swallowed the warning on exactly
> that launch. Source incident: cycle 1444 audit M1 (re-verified live in
> cycle 1446).
