---
score_cap:
  - criterion: "Retro never dispatches BridgeRequest.Model == \"auto\" — the sentinel resolves to a concrete tier before launch"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/phases/retro -run '^TestRun_AutoModel_ResolvedBeforeDispatch$' -count=1"
  - criterion: "An explicitly configured model tier reaches the bridge unchanged"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/retro -run '^TestRun_ExplicitModel_PassesThroughUnchanged$' -count=1"
  - criterion: "Degraded resolution (no model_tier_default, or no profile at all) still never forwards the sentinel"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/retro -run '^TestRun_AutoModel_(ProfileWithoutTier_ResolvesToDefaultNotAuto|NoProfile_NeverDispatchesSentinel)$' -count=1"
---

# Eval: Normalize Retro's auto model before bridge dispatch

> Pins the dispatched `core.BridgeRequest.Model` for the retro phase. Retro is a
> hand-rolled runner: it never passes through `runner.BaseRunner`, so it never
> reaches the single dispatch resolver (`llmroute.Resolve` → the `autoExpand`
> seam → `resolvellm.Resolve`) that expands the `"auto"` sentinel for every
> other phase. `cmd_cycle.go` supplies the sentinel correctly — every sibling
> phase's `DefaultModel()` does the same — so the fix belongs in the phase, not
> the wiring. A literal `"auto"` reaching the CLI is a routing defect (claude -p
> rejects it outright), which is why the caps are keyed to the DISPATCHED
> request rather than to `resolvellm` in isolation. Source incident: cycle 1568
> (inbox item `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-sentinel-dispatch | unset model resolves to the profile tier | 7/10 | `go test ./internal/phases/retro -run '^TestRun_AutoModel_ResolvedBeforeDispatch$'` |
| explicit-tier-preserved | configured tier passes through unchanged | 6/10 | `go test ./internal/phases/retro -run '^TestRun_ExplicitModel_PassesThroughUnchanged$'` |
| degraded-resolution | no tier / no profile still never `"auto"` | 6/10 | `go test ./internal/phases/retro -run '^TestRun_AutoModel_(ProfileWithoutTier…|NoProfile…)$'` |
