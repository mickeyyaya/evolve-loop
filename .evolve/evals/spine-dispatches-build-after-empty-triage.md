---
score_cap:
  - criterion: "An empty triage top_n terminates the cycle on the COMPOSED dispatch path — the orchestrator dispatches no phase after triage, not merely the router proposing PhaseEnd"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath ./acs/cycle1623"
  - criterion: "A committed top_n still dispatches the spine on the COMPOSED path — the anti-no-op control for the composed gate"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1623_006_CommittedTopNStillDispatchesTheSpine ./acs/cycle1623"
  - criterion: "The early-exit authority (core.enforceNext) itself honours a known-empty commitment while keeping the ship-planned invariant and failing open on an unknown commitment"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestEnforceNext_EmptyTriageCommitmentIsALegalTermination|TestCanTerminateEarly_ShipPlannedInvariantIntact' ./internal/core"
  - criterion: "An empty triage top_n terminates the cycle instead of dispatching the tdd/build/audit spine"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine ./acs/cycle1623"
  - criterion: "A non-empty triage top_n still advances the spine — the gate keys on the committed count, not on the triage phase"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1623_002_CommittedTopNStillAdvancesTheSpine ./acs/cycle1623"
  - criterion: "A failed inbox claim leaves the item in the inbox root — ownership failure never strands work"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1623_003_ClaimFailureLeavesInboxItemInPlace ./acs/cycle1623"
  - criterion: "The cycle's ACS predicate package is git-tracked, so the predicate tree and the ship tree agree"
    max_if_missing: 6
    evidence: "git ls-files --error-unmatch go/acs/cycle1623/predicates_test.go"
---

# Eval: the spine must not dispatch on an empty triage commitment

> Pins the triage→next-phase edge. In cycle 1623 the atomic inbox claim for
> `agy-tier-map-single-source` failed under the phase sandbox, so triage
> committed `"top_n": []` — and the router advanced anyway:
> `routing-decision-3.json` returned `tdd` with reason `conditional-pin:tdd`,
> then `routing-decision-4.json` returned `build` with reason `spine:build`,
> then `audit`. Two full phases plus an audit ran against a task no phase was
> authorized to own, and the audit correctly returned FAIL. Scout had
> catalogued this exact defect in the same cycle
> (`scout-report.md:69`, `spine-dispatches-build-after-empty-triage`) and
> deferred it; it then fired on the cycle that named it.
>
> The second criterion is the anti-no-op control and is deliberately capped
> HIGHER than the first: a "fix" that simply terminates after every triage
> would satisfy criterion 1 while bricking every productive cycle. The gate
> must key on the committed count reaching the router from
> `triage-decision.json`, which is why the predicate drives the composed
> `router.Digest` → `router.Route` path over a real on-disk workspace rather
> than a hand-built signal literal (lesson inst-L1563a).
>
> Criteria 3 and 4 pin the collateral findings from the same audit: inbox
> ownership must survive a claim failure (today true only by statement
> ordering at `go/internal/inboxmover/inboxmover.go:209-215`, asserted by
> nothing), and a cycle's predicate package must be git-tracked or the audit's
> predicate execution tree carries inputs absent from the ship tree — the
> literal gate reason cycle 1623 failed on (cycle-93 lesson).
>
> **Round-2 addendum — why the first three criteria were not enough.** The
> round-2 fix satisfied criterion "empty-commitment-terminates" and the audit
> still FAILed: `router.Route` returned `PhaseEnd`, but `Route`'s decision is
> only a PROPOSAL. `go/internal/core/cyclerun_select.go:103` routes it through
> `Orchestrator.enforceNext`, whose `PhaseEnd` branch
> (`go/internal/core/routing_dispatch.go:59-63`) asks
> `StateMachine.CanTerminateEarly(current, shipPlanned)` — which returns
> `false` unconditionally when ship is planned
> (`go/internal/core/statemachine.go:230-233`). Cycle 1623's own clamped plan
> schedules ship, so the proposal was discarded and the orchestrator dispatched
> `tdd` anyway. The predicate stayed GREEN through the entire live defect
> because it asserted one layer ABOVE the authority that decides the next
> phase. The three criteria added at the top of this eval therefore grade the
> decision where it is CONSUMED — a whole `RunCycle`, and the `enforceNext`
> table itself — and they are capped at 9/10, above the router-layer criterion
> they supersede. A GREEN predicate at the producing layer is not evidence
> about the consuming layer.
>
> Source incident: cycle 1623 (audit round 1 findings H2, M2, M1; audit round 2
> finding H1).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| composed-path-terminates | The ORCHESTRATOR dispatches nothing after an empty `top_n` | 9/10 | `go test -tags acs -run TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath` |
| composed-path-anti-no-op | A committed `top_n` still dispatches tdd+build on the composed path | 9/10 | `go test -tags acs -run TestC1623_006_CommittedTopNStillDispatchesTheSpine` |
| authority-honours-commitment | `enforceNext` terminates on a known-empty commitment, stays blocked on an unknown one, never leaks past build | 9/10 | `go test -run TestEnforceNext_EmptyTriageCommitmentIsALegalTermination ./internal/core` |
| empty-commitment-terminates | Empty `top_n` must not dispatch tdd/build/audit (router layer) | 8/10 | `go test -run TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine` |
| committed-still-advances (anti-no-op) | One committed task must still advance the spine | 9/10 | `go test -run TestC1623_002_CommittedTopNStillAdvancesTheSpine` |
| claim-failure-atomicity | Failed claim leaves the inbox item in place | 7/10 | `go test -run TestC1623_003_ClaimFailureLeavesInboxItemInPlace` |
| predicate-tree-is-ship-tree | Cycle ACS package git-tracked | 6/10 | `git ls-files --error-unmatch go/acs/cycle1623/predicates_test.go` |

## Not covered here

Audit finding **H1** — the phase sandbox denies
`<project_root>/.evolve/inbox/processing/cycle-<N>`, so no sandboxed lane can
claim any inbox item — is deliberately absent from this eval. Its fix site
(`go/internal/bridge/sandbox_wrap.go:209` and `go/internal/adapters/sandbox/`)
is inside `guards.ProtectedSurfaceManifest`; a lane cannot land the edit, so a
score_cap that no cycle can ever clear would cap every future audit forever.
H1 is routed to the console owner as a manual checklist in
`.evolve/runs/cycle-1623/test-report.md`.
