---
score_cap:
  - criterion: "a verified submit_wedged prompt short-circuits to ExitArtifactTimeout before consuming any of the artifact-wait silence budget"
    max_if_missing: 9
    evidence: "go -C go test -count=1 -run '^TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget$' ./internal/bridge"
  - criterion: "a verified-clean submission and a generically silent pane are never classified as delivery failures"
    max_if_missing: 9
    evidence: "go -C go test -count=1 -run '^TestTmuxREPL_(CleanSubmit_NeverClassifiesDeliveryFailure|SilentPaneTimeout_NotClassifiedAsDeliveryFailure)$' ./internal/bridge"
  - criterion: "a wedged one-shot nudge names its site and classification in the terminal artifact-timeout marker"
    max_if_missing: 7
    evidence: "go -C go test -count=1 -run '^TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker$' ./internal/bridge"
  - criterion: "recovery stays bounded: re-sends capped at three, dispatcher relaunch capped at one"
    max_if_missing: 8
    evidence: "go -C go test -count=1 -run '^TestTmuxREPL_Nudge(Unsubmitted_ResendBounded|Submitted_NoResend)$' ./internal/bridge && go -C go test -count=1 -run '^TestOrchestrator_PhaseArtifactTimeout_(RetriesAndRecovers|AbortsAfterCap)$' ./internal/core"
---

# Eval: retrospective delivery failure reaches the bounded relaunch

> Locks the consumer half of the tmux submit-verification signal. The producer
> (`go/internal/bridge/driver_tmux_submitverify.go`) has classified a stuck
> input line as `interaction.ResultSubmitWedged` since cycle 1526, but both
> consumer sites in `driver_tmux_repl.go` — the prompt-paste delivery and the
> one-shot idle nudge — pipe `verifySubmitted`'s outcome straight into
> `recordSubmitVerify`, which appends to the phase interaction ledger and
> returns nothing usable for control flow. A delivery failure detected in
> milliseconds is therefore indistinguishable, at the control-flow level, from
> a healthy launch that simply never speaks.
>
> What that cost. In cycle 1510 a retrospective launch logged `prompt
> delivered`, produced zero tokens and zero cost, and then waited two full
> 900-second stop-review intervals before dying with `ExitArtifactTimeout` —
> half an hour spent on a pane whose prompt was already known to be
> unsubmitted. The dispatcher's existing one-relaunch recovery
> (`cyclerun_dispatch.go`, `IsInfraTeardownError` over `ErrArtifactTimeout`)
> was reachable the entire time; nothing routed the evidence to it.
>
> The fix is a short-circuit into the EXISTING bounded outcome, never a new
> retry loop and never a new sentinel. Rows 2 and 4 are why this eval is
> mostly negatives: a classifier that fires without an evidenced
> submit-verification failure, or a re-send loop that stops being bounded,
> would each be strictly worse than the bug — every ordinary slow phase would
> become a bridge relaunch, or a wedged pane would be hammered indefinitely.
> Source incident: cycle 1510 (`.evolve/runs/cycle-1510/retrospective-launch-error.txt`,
> `retrospective-interactions.ndjson`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| short-circuit | `submit_wedged` prompt exits immediately; the binding test counts two-second artifact-wait polls, so relabelling the eventual timeout cannot satisfy it | 9/10 | `go test -run TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget ./internal/bridge` |
| no-false-positive | Verified-clean submissions exit `ExitOK` with no timeout marker; generically silent panes keep the generic stop-review reason | 9/10 | `go test -run 'TestTmuxREPL_(CleanSubmit_…\|SilentPaneTimeout_…)' ./internal/bridge` |
| nudge-site | The second consumer site names site + classification in its marker instead of the generic stall reason | 7/10 | `go test -run TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker ./internal/bridge` |
| bounded-recovery | Re-sends ≤ 3 and exactly one dispatcher relaunch — the anti-weakening floor against greening the short-circuit by loosening a bound | 8/10 | `go test -run 'TestTmuxREPL_Nudge…' ./internal/bridge` + `go test -run 'TestOrchestrator_PhaseArtifactTimeout_…' ./internal/core` |
