---
score_cap:
  - criterion: "a resumed cycle continues at its recorded rung, and a rung raised in the resumed correction loop reaches the next phase"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestResumedCycle_ContinuesAtItsRecordedRung|TestResumedCorrection_TheRaisedRungReachesTheNextPhase)$' ./internal/core"
  - criterion: "a quota-paused resume writes the raised rung to its checkpoint and the next resume continues there; a shadow ladder leaves the dispatch unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestResumedCorrection_AQuotaWallPausesAtTheRaisedRungAndTheNextResumeContinuesThere|TestResumedCorrection_ShadowQuotaPauseLeavesTheNextResumeDispatchUnchanged)$' ./internal/core"
  - criterion: "ladder progress made after the kept rung was adopted survives the next dispatch and the quota checkpoint of the same resumed run"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesTheNextDispatch|TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesAQuotaPause|TestResumedRun_LadderProgressAfterTheResumedPhaseSurvivesTheNextDispatch|TestResumedRun_AQuotaWallAfterLaterLadderProgressCheckpointsTheLaterRung)$' ./internal/core"
---

# Eval: a resumed run keeps its retry ladder state

> Audit round 1 of cycle 2486 rejected the first build (H1): `adoptResumedLadder` never consumed the kept snapshot, so a later raise was reverted at the next dispatch and the quota checkpoint kept the stale rung.
>
> The resumed correction loop of `reviewResumedDeliverable` raises the ladder on a copy of the cycle state. A later phase and a quota-pause checkpoint of the same resumed run start again from the old rung. This eval pins the rung across both. Source: plan docs/plans/retry-ladder-2026-10.md section 5, gap 2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| recorded-rung | a resumed phase and the next phase carry the recorded or raised rung | 6/10 | `go test -run TestResumedCycle_ContinuesAtItsRecordedRung` |
| quota-pause | the quota checkpoint carries the raised rung and the next resume continues there | 7/10 | `go test -run TestResumedCorrection_AQuotaWallPausesAtTheRaisedRung...` |
| later-progress | a raise made after the adoption survives the next dispatch and the quota pause | 8/10 | `go test -run TestAdoptedResumedLadder_` |
