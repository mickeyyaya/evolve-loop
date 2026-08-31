---
score_cap:
  - criterion: "The transient pattern is resolved once from the launched CLI's manifest in newAutoResponder (family-agnostic), not recompiled per 2s poll"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAutoResponder_TransientRegex' ./internal/bridge"
  - criterion: "A pane parked on a recognized transient upstream error stops through the existing exit-81 path after its 60s dwell, before the artifact-timeout reviewer is consulted"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_(TransientDwell_EnforceStopsBeforeTheArtifactReviewer|TransientPaneSkipsFullArtifactTimeout)$' ./internal/bridge"
  - criterion: "The dwell does not fire inside its 60s window and resets on any non-matching frame, so an intermittent blip never kills a phase"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_TransientDwell_(DoesNotFireBeforeSixtySeconds|ResetsOnNonMatchingFrame)$' ./internal/bridge"
  - criterion: "A BUSY pane is never preempted by the transient dwell (stop-review prime directive, mirroring fatalPaneVerdict's ev.Busy guard)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted$' ./internal/bridge"
  - criterion: "The ADR-0044 RecoveryStage dial gates the action (off=legacy+unclassified, shadow=observe-only) while shadow/enforce leave durable would_fast_fail/fast_failed records in <workspace>/<phase>-interactions.ndjson"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_TransientDwell_(ShadowObservesWithoutActing|EnforceRecordsFastFailed|OffStageIsLegacy)$' ./internal/bridge"
  - criterion: "The fired dwell reuses the existing !completed machinery — ReviewStop verdict, escalation report with pane evidence, transient=true marker line, exit 81 — and adds no new exit code"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts$' ./internal/bridge"
  - criterion: "A deliberate re-dispatch delay applies on the transient shortcircuit path only, never on the ordinary artifact timeout"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRunTmuxREPL_TransientDwell_(EnforceDelaysRedispatch|NoDelayOnOrdinaryTimeout)$' ./internal/bridge"
---

# Eval: Transient-artifact-timeout shortcircuit (the silence budget)

> Pins the in-wait transient shortcircuit introduced in cycle 1580. Before it,
> `classifyTransientPane` was consulted only AFTER the artifact wait had already
> timed out, where it merely annotated the marker line — nothing read it during
> the wait. A session parked on "API Error: 529 Overloaded … usually temporary"
> therefore burned the entire silence budget: 3 of 4 observed router stalls
> (cycles 1523/1524/1526) spent ~600s each on a pane that stated its own cause.
> The fix is a manifest-sourced dwell tracker on the ~2s poll that, after 60
> consecutive seconds on a non-busy pane and at the enforce rung of the ADR-0044
> dial, sets a ReviewStop verdict and falls into the EXISTING exit-81 machinery.
> These caps keep every guard on that path alive: the dwell (so one blip cannot
> kill a phase), the busy guard (the cycle-254/255 false-FAIL prime directive),
> the stage dial + would/did telemetry (so the soak can measure false
> positives), and the exit-code contract (81 stays the single artifact-timeout
> code — transient-bridge-retry AC-1 depends on it).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| manifest-sourced-once | transientRegex resolved in `newAutoResponder`, per family | 6/10 | `go test -run '^TestAutoResponder_TransientRegex' ./internal/bridge` |
| shortcircuit-fires | 60s dwell preempts the full silence budget | 9/10 | `go test -run '^TestRunTmuxREPL_(TransientDwell_EnforceStopsBeforeTheArtifactReviewer\|TransientPaneSkipsFullArtifactTimeout)$' ./internal/bridge` |
| dwell-holds-and-resets | no fire inside 60s; reset on any non-match | 8/10 | `go test -run '^TestRunTmuxREPL_TransientDwell_(DoesNotFireBeforeSixtySeconds\|ResetsOnNonMatchingFrame)$' ./internal/bridge` |
| busy-never-killed | working agent is never preempted | 8/10 | `go test -run '^TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted$' ./internal/bridge` |
| stage-dial-and-evidence | off/shadow/enforce + durable would/did records | 7/10 | `go test -run '^TestRunTmuxREPL_TransientDwell_(ShadowObservesWithoutActing\|EnforceRecordsFastFailed\|OffStageIsLegacy)$' ./internal/bridge` |
| reuse-not-duplicate | ReviewStop → existing report/marker/exit 81 | 8/10 | `go test -run '^TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts$' ./internal/bridge` |
| scoped-redispatch-delay | delay on the shortcircuit path only | 6/10 | `go test -run '^TestRunTmuxREPL_TransientDwell_(EnforceDelaysRedispatch\|NoDelayOnOrdinaryTimeout)$' ./internal/bridge` |
