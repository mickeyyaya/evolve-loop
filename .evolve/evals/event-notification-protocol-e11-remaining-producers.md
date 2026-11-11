---
score_cap:
  - criterion: "evolve wave next emits wave.started and wave.ended through the real launch path"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWaveNext_EmitsWaveStarted|TestWaveNext_EmitsWaveEndedForTheWaveItCloses|TestWaveNext_TheFirstWaveEndsNothing|TestWaveNext_ADryRunAndAFailedBoundaryEmitNothing)$' ./cmd/evolve"
  - criterion: "ci watch emits ci.completed and a red run is a WARN with CI_RUN_RED"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCIWatch_' ./internal/ciwatch"
  - criterion: "the evolve ci watch verb publishes its events to the channels"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCIWatchVerb_' ./cmd/evolve"
  - criterion: "inbox.claimed and inbox.released are emitted by the claim and release paths the loop runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestInboxLifecycle_' ./internal/inboxmover && go test -count=1 -run '^TestClaims_ReleasePublishesInboxReleasedThroughTheEnginesCenter$' ./internal/loopwave && go test -count=1 -run '^TestInboxLifecycle_TheHostClaimer' ./cmd/evolve"
  - criterion: "runCIWatch fails closed when the root or the run lookup fails"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestRunCIWatch_AWorkingDirectoryFaultExitsIOWhenNoRootIsGiven|TestRunCIWatch_AFailedRunLookupExitsIOAndNamesTheWorkflow)$' ./cmd/evolve"
  - criterion: "the five kinds, the ci module and CI_RUN_RED are in the closed sets"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRegistry_' ./internal/signalcenter"
---

# Eval: Event channels E11 remaining producers

> Pins the five producers of plan docs/plans/event-notification-protocol-2026-10.md component E11:
> `wave.started`, `wave.ended`, `inbox.claimed`, `inbox.released` and `ci.completed`. The inbox producers sit in the unprotected
> `inboxmover/claim_pending.go` and `inboxmover/claims.go`, which already receive `Options.Signals`; no protected file changes.
> Source: inbox item `event-notification-protocol-e11-remaining-producers`, cycle 2321. The audit of the first build (H1, M1)
> found that this eval had narrowed acceptance 2 away; it covers all five acceptance criteria again. Each
> predicate drives the production caller (`runWaveWith`, `runCIWatch`, `hostInboxClaimer`, the loopwave engine), never
> the seam alone, so a producer that no entry point reaches stays red.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wave-producers | wave.started and wave.ended reach the loop channel | 6/10 | `go test -run TestWaveNext_ ./cmd/evolve` |
| ci-producer | green INFO, red WARN with CI_RUN_RED | 6/10 | `go test -run TestCIWatch_ ./internal/ciwatch` |
| ci-caller-proof | the verb publishes to the signals and errors channels | 6/10 | `go test -run TestCIWatchVerb_ ./cmd/evolve` |
| inbox-producers | claim and release paths emit inbox.claimed and inbox.released | 6/10 | `go test -run TestInboxLifecycle_ ./internal/inboxmover` |
| ci-fail-closed | a root fault or a failed run lookup exits IO | 5/10 | `go test -run TestRunCIWatch_A ./cmd/evolve` |
| registry | closed sets hold the new names | 7/10 | `go test -run TestRegistry_ ./internal/signalcenter` |
