---
score_cap:
  - criterion: "the classified delivery-failure cause survives Engine.Launch into the phase error, still wrapped in core.ErrArtifactTimeout"
    max_if_missing: 8
    evidence: "go -C go test -count=1 -run '^TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause$' ./internal/bridge"
  - criterion: "the terminal <phase>-failure-diag.json exposes the delivery-failure cause as its own machine-readable field"
    max_if_missing: 8
    evidence: "go -C go test -count=1 -run '^TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable$' ./internal/core"
  - criterion: "a generic silent-agent timeout and a non-timeout failure both leave the delivery-failure field empty"
    max_if_missing: 9
    evidence: "go -C go test -count=1 -run '^TestWritePhaseFailureDiag_(GenericSilence|NonTimeoutFailure)_NoDeliveryFailureAttribution$' ./internal/core"
  - criterion: "pre-existing artifact-timeout evidence is not weakened by the new field"
    max_if_missing: 7
    evidence: "go -C go test -count=1 -run '^TestEngineLaunch_(ArtifactTimeout_ErrorCarriesWaitAndExtends|NonTimeoutExit_CauseUnchanged)$' ./internal/bridge && go -C go test -count=1 -run '^TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning$' ./internal/core"
---

# Eval: terminal retro delivery failure leaves machine-readable evidence

> Locks the evidence half of the cycle-1510 retro delivery defect. When the
> bounded relaunch is exhausted, the cycle dies through
> `writePhaseFailureDiag` (`go/internal/core/failure_learning.go`), whose
> `phaseFailureDiag` struct carries only a flat `error_message` and an exit
> code derived from `errors.Is(phaseErr, ErrArtifactTimeout)`. The reason the
> launch actually failed — the driver had verified, in milliseconds, that the
> prompt was never submitted — existed nowhere machine-readable: it survived
> only as a substring of free text, if the driver stderr survived at all
> (`engine.go` discards it past the `<agent>-launch-error.txt` write).
>
> Why a typed field and not a substring. "The prompt was never delivered" and
> "the agent went quiet" share an exit code (81) and share the
> `artifact-timeout:` marker shape, but they have opposite remedies: relaunch
> the pane versus raise `bridge.phase_artifact_timeout_s` for that phase. Every
> downstream reader — failure learning, the failure adviser, an operator
> triaging a repeat — has to make that call, and none of them should be
> parsing prose to do it.
>
> Row 3 is the load-bearing one and the reason this eval leans negative: a
> field populated on every exit-81 carries no information at all, and would
> point an operator at a wedged pane whose real problem was too small an
> artifact budget. Row 4 guards the shared cause-selection path both tasks
> touch (driver marker → `artifactTimeoutSummary` → `phaseErr` → failure
> diagnostic), where the cheapest way to green a new assertion is to relax an
> old one. Source incident: cycle 1510
> (`.evolve/runs/cycle-1510/retrospective-launch-error.txt`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| bridge-boundary | Classified cause reaches `phaseErr` via the existing marker path, still `core.ErrArtifactTimeout` so the one relaunch is preserved | 8/10 | `go test -run TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause ./internal/bridge` |
| typed-diagnostic | `<phase>-failure-diag.json` carries the cause as its own field, naming site and classification | 8/10 | `go test -run TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable ./internal/core` |
| no-false-attribution | Generic silence and non-timeout failures leave the field empty; the flat cause is preserved verbatim | 9/10 | `go test -run 'TestWritePhaseFailureDiag_…_NoDeliveryFailureAttribution' ./internal/core` |
| evidence-not-weakened | Existing waited/extends fields, unchanged non-timeout causes, and the diagnostic's non-interference with retry/failure-learning all still hold | 7/10 | `go test -run 'TestEngineLaunch_…' ./internal/bridge` + `go test -run TestArtifactTimeoutEndToEnd_… ./internal/core` |
