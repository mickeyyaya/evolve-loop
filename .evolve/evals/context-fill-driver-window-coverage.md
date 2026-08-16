---
score_cap:
  - criterion: "Known supported non-claude driver families (codex, agy — bare and -tmux variant) yield a finite, conservative context-fill window instead of the unmeasured sentinel"
    max_if_missing: 7
    evidence: "cd go && go test -run TestFillTelemetry_EffectiveWindowMeasuredNonClaudeFamilies ./internal/tokenusage"
  - criterion: "Unsupported/unmeasured driver families (ollama, unknown CLIs, adjacent-looking names like codexx/agyx, malformed identities) stay explicitly unmeasured — never a guessed window"
    max_if_missing: 8
    evidence: "cd go && go test -run TestFillTelemetry_EffectiveWindowUnsupportedFamiliesStayUnmeasured ./internal/tokenusage"
  - criterion: "DefaultResolver actually stamps the new mapped window onto a real codex Window (wiring, not just the table lookup)"
    max_if_missing: 7
    evidence: "cd go && go test -run TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver ./internal/tokenusage"
  - criterion: "A measured launch on an unmapped family loses only its FILL reading, never its recovered usage, and never fabricates a window"
    max_if_missing: 7
    evidence: "cd go && go test -run TestFillTelemetry_UnmeasuredResolveCarriesSentinelForUnknownDriver ./internal/tokenusage"
  - criterion: "Widening the driver-window table does not move the already-calibrated claude family"
    max_if_missing: 6
    evidence: "cd go && go test -run TestFillTelemetry_EffectiveWindowClaudeFamilyUnchanged ./internal/tokenusage"
---

# Eval: Context-fill driver window coverage

> Pins the per-family effective-window table introduced in cycle-1482 for the
> `context-fill-telemetry-and-cap` inbox item. `EffectiveWindow` previously
> mapped exactly one family (claude); every non-claude driver — including
> codex and agy, whose real advertised windows are documented — degraded to
> the unmeasured sentinel forever. This eval enforces both halves of the fix
> together: a conservative, evidence-backed window for the families that have
> one, and a hard "stay unmeasured" floor for every family that doesn't,
> including adjacent-looking impostor names (`codexx`, `agyx`) that a naive
> prefix match would swallow. Source: scout-report.md cycle-1482 Task 1;
> research backing `.evolve/inbox/2026-07-30T13-02-00Z-context-fill-telemetry-and-cap.json`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| positive-coverage | codex/agy (+ -tmux variants) get a finite, conservative window | 7/10 | `go test -run TestFillTelemetry_EffectiveWindowMeasuredNonClaudeFamilies` |
| negative-floor | ollama, unknown, and impostor-adjacent names stay unmeasured | 8/10 | `go test -run TestFillTelemetry_EffectiveWindowUnsupportedFamiliesStayUnmeasured` |
| resolver-wiring | DefaultResolver stamps the new window on a real codex Window | 7/10 | `go test -run TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver` |
| anti-fabrication | measured usage on an unmapped family never invents a window | 7/10 | `go test -run TestFillTelemetry_UnmeasuredResolveCarriesSentinelForUnknownDriver` |
| regression | claude's existing calibration survives the widening | 6/10 | `go test -run TestFillTelemetry_EffectiveWindowClaudeFamilyUnchanged` |
