---
score_cap:
  - criterion: "A cycle whose triage writes top_n [] while <ProjectRoot>/.evolve/inbox holds a claimable lane item dispatches no tdd/build/audit/ship phase, records a named terminal reason distinct from triage-empty-commitment, never PASSes, and is never credited as IsTriageNoWorkResult — on the fresh AND the resumed dispatch root"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^(TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation|TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation)$' ./internal/core"
  - criterion: "The legitimate empty-inbox case (empty inbox dir, absent inbox dir, or only console-routed items) still classifies SKIPPED via recordPlannedNoWorkOutcome with reason triage-empty-commitment and a cleaned worktree"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_EmptyInboxIsPlannedNoWork$' ./internal/core"
  - criterion: "Committed work beside a still-populated inbox dispatches tdd and build and PASSes (the gate keys on empty-commitment AND claimable work, never on the inbox alone)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation$' ./internal/core"
  - criterion: "evolve cycle-health classifies a historical dossier with tasks [] and any tdd/build/audit/ship phase as a dossier_commitment anomaly naming the phase, and stays quiet on empty commitments that ended at triage, legacy records without the tasks field, committed work, and a missing dossier"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestCycleHealth_EmptyCommitmentDossierWithImplementationIsAnomaly|TestCycleHealth_DossierCommitmentSignalStaysQuietOnHealthyShapes)$' ./cmd/evolve"
---

# Eval: A triage that commits to nothing must not spend the spine — and a claim failure must not be credited as no-work

> Pins the fourth acceptance line of the cycle-1623 P0 (split out as inbox
> `2026-09-12T10-00-00Z-triage-empty-commitment-still-dispatches-spine.json`,
> P1, weight 0.86). Cycle 1623: triage could not claim its selected item,
> wrote `triage-decision.json` with `top_n []` and one deferral, and the
> orchestrator dispatched twelve more phases and shipped 922 lines against no
> committed task. The round-2 gate (`decideTriageTermination` +
> `cyclerun_select.go`'s PhaseTriage branch) stops every explicit empty
> commitment before tdd — but it stops it as `triage-empty-commitment`, the
> LEGITIMATE planned-no-work disposition, without asking whether the inbox
> still held claimable work. A claim race between fleet lanes, a loader that
> drops an item, or an agent that mis-writes the decision are therefore all
> credited as "nothing to do". The distinguishing input is "was there work to
> claim". Source incident: cycle 1623 (`.evolve/runs/cycle-1623.reset-*/
> triage-decision.json`); RED authored in cycle 1652.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| claimable-empty-commitment | claimable inbox + top_n [] stops before tdd with a named non-no-work reason, both roots | 3/10 | `go test -run 'TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation\|TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation' ./internal/core` |
| legitimate-no-work-pin | empty/absent/console-only inbox stays SKIPPED via recordPlannedNoWorkOutcome | 5/10 | `go test -run TestRunCycle_EmptyInboxIsPlannedNoWork ./internal/core` |
| anti-no-op | committed work beside a populated inbox still advances | 4/10 | `go test -run TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation ./internal/core` |
| historical-anomaly | 1623-shaped dossier is a `dossier_commitment` cyclehealth anomaly; healthy shapes stay quiet | 6/10 | `go test -run 'TestCycleHealth_EmptyCommitmentDossierWithImplementationIsAnomaly\|TestCycleHealth_DossierCommitmentSignalStaysQuietOnHealthyShapes' ./cmd/evolve` |
