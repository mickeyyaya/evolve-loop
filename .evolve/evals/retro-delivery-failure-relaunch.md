---
score_cap:
  - criterion: "A typed submit_wedged delivery failure triggers exactly one fresh Retro dispatch on the real Phase.Run route"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/retro -run '^TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce$' -count=1"
  - criterion: "A generic artifact timeout with no typed delivery classification does not relaunch"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/phases/retro -run '^TestRun_GenericArtifactTimeout_DoesNotRelaunch$' -count=1"
  - criterion: "The retro phase package stays green as a whole (verdict mapping + profile-CLI dispatch pins + relaunch contract)"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/retro -count=1"
---

# Eval: Relaunch Retro once on typed delivery failure

> Pins the delivery-failure consumer on the Retro dispatch path. The bridge
> already classifies a wedged submission (`submit_wedged`, zero tokens, prompt
> still parked at the pane) and core already preserves it as a machine-readable
> `delivery_failure` diagnostic — but `retro.Phase.Run` swallows every bridge
> error into a FAIL verdict with a nil error (the deliberate GAP-9 self-healing
> return), so Retro is structurally excluded from the generic one-relaunch
> self-heal every other phase gets from `cyclerun_dispatch.go`. Cycles
> 1505/1510/1517 lost their retrospective entirely to this. The contract is
> narrow on purpose: exactly ONE extra launch, and only when the failure is
> typed — a blanket retry of every `ErrArtifactTimeout` is the over-fix this
> eval's second cap rejects. Source incident: cycle 1568 (inbox item
> `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| typed-relaunch | one fresh dispatch on `submit_wedged` | 8/10 | `go test ./internal/phases/retro -run '^TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce$'` |
| no-blanket-retry | generic timeout stays single-launch/FAIL | 7/10 | `go test ./internal/phases/retro -run '^TestRun_GenericArtifactTimeout_DoesNotRelaunch$'` |
| package-regression | retro package suite green | 6/10 | `go test ./internal/phases/retro` |
