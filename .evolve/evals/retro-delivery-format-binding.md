---
score_cap:
  - criterion: "The classified submit_wedged cause produced by a REAL Engine.Launch is parsed by the consumer's own classifier (core.DeliveryFailureCause), not merely present as substrings in the error text"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier$' ./internal/bridge | grep -q -- '--- PASS: TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier'"
  - criterion: "NEGATIVE — a real generically silent pane timeout is NOT classified as a delivery failure, so an ordinary slow phase never triggers a bridge relaunch"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause$' ./internal/bridge | grep -q -- '--- PASS: TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause'"
  - criterion: "Retro's consumer relaunches exactly once on a classified delivery failure and not at all on a generic artifact timeout"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRun_(SubmitWedgedDeliveryFailure_RelaunchesOnce|GenericArtifactTimeout_DoesNotRelaunch)$' ./internal/phases/retro | grep -c -- '--- PASS: TestRun_' | grep -qx 2"
  - criterion: "The delivery classification stays machine-readable in the persisted failure diagnostic (delivery_failure), so failure-learning keeps it as data rather than discarded stderr"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable$' ./internal/core | grep -q -- '--- PASS: TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable'"
---

# Eval: bind the bridge submit_wedged format to retro's delivery-failure consumer

> Cycle 1510's retrospective launch logged "prompt delivered", produced zero tokens, and
> then burned two full 900s stop-review intervals before dying with a bare artifact
> timeout — while the submit-verify guard had already classified that pane as
> `submit_wedged` within milliseconds. The fix made the classification ride the existing
> `artifact-timeout:` marker (`driver_tmux_repl.go` `writeArtifactTimeoutMarker`, lifted by
> `artifactTimeoutSummary` in `engine.go`) into the error `Engine.Launch` returns, and
> `retro.go:203` keys its single relaunch on `core.DeliveryFailureCause(err) != ""`.
>
> That chain is a string contract across three files, and nothing pins the two ends
> together. The bridge-side test asserts only that `submit_wedged`, `prompt`, and
> `resends=` appear somewhere in the error text; the retro-side tests build the error from
> a hand-copied literal. So a producer-side change to the `reason=%q` framing — different
> quoting, a renamed key, a reordered field — leaves both suites green while
> `DeliveryFailureCause` starts returning `""`, and every wedged retro silently decays back
> into the cycle-1510 shape: a generic artifact timeout, no relaunch, the whole silence
> budget burned. This eval binds the REAL produced error to the consumer's own classifier,
> and keeps the false-positive half pinned so ordinary silence is never relaunched. Source
> incidents: cycle 1510 (the original stall), cycle 1591 (this binding).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| producer-consumer-binding | Real Launch error parses under `DeliveryFailureCause` | 9/10 | `-run TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier` |
| negative-no-overfire | Real generic silence timeout classifies as "" | 9/10 | `-run TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause` |
| consumer-relaunch | Retro relaunches once, and only on the classified cause | 8/10 | `-run TestRun_(SubmitWedged…|GenericArtifactTimeout…)` |
| diagnostic-machine-readable | `delivery_failure` survives into the failure diag | 7/10 | `-run TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable` |

## Acceptance Criteria (code-graded)

### AC1: a real bridge submit_wedged error is classified by the consumer's classifier [code]
```bash
cd go && go test -count=1 -v -run '^TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier$' ./internal/bridge | grep -q -- '--- PASS: TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier'
```
Expected: exit 0

### AC2 (negative): a real generic silence timeout is not a delivery failure [code]
```bash
cd go && go test -count=1 -v -run '^TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause$' ./internal/bridge | grep -q -- '--- PASS: TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause'
```
Expected: exit 0

### AC3: retro relaunches exactly once, and only on the classified cause [code]
```bash
cd go && go test -count=1 -v -run '^TestRun_(SubmitWedgedDeliveryFailure_RelaunchesOnce|GenericArtifactTimeout_DoesNotRelaunch)$' ./internal/phases/retro | grep -c -- '--- PASS: TestRun_' | grep -qx 2
```
Expected: exit 0

### AC4: the classification stays machine-readable in the persisted failure diag [code]
```bash
cd go && go test -count=1 -v -run '^TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable$' ./internal/core | grep -q -- '--- PASS: TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable'
```
Expected: exit 0
