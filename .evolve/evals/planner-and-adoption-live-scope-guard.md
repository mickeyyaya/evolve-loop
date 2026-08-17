---
score_cap:
  - criterion: "A registry binding whose scope id has no live pending item is refused for dispatch, logged, and released"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_004_AdoptionRefusesGhostScopeAndReleases$' ./acs/cycle1507"
  - criterion: "A binding whose scope id IS a live pending inbox item is still adopted and never released (anti-overreach)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_005_AdoptionAcceptsLivePendingItem$' ./acs/cycle1507"
  - criterion: "An item claimed into processing/cycle-N counts as LIVE — an in-flight lane never loses its binding"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_006_AdoptionAcceptsClaimedProcessingItem$' ./acs/cycle1507"
  - criterion: "A continuation stamped on this cycle's processing claim still resolves first (G1 semantics untouched)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_007_ClaimStampedContinuationStillWins$' ./acs/cycle1507"
---

# Eval: planner and adoption live-scope guard

> Pins the read-side safety net of the continuation registry. The scope-keyed registry
> read (`inboxmover.ResolveContinuationForScope`, injected at `cmd/evolve/cmd_cycle.go`
> into `core.WithContinuationResolver`) is the ONE seam both the wave planner's lane-scope
> minting and the post-triage adoption path go through, and it trusted a registry hit
> without ever asking whether the scope id still names a live pending item. Batch-20260816d
> cycle-1497 pinned the source: a CONSUMED scope was re-dispatched with no adoption event
> and no carryover entry (both verified empty) — the planner mints lanes from bindings, so
> the registry is a first-class lane SOURCE. The guard is the belt that holds even if a
> release call is ever missed at a pool-exit path; the anti-overreach criteria keep it from
> trading a re-dispatch defect for a salvage-loss defect. "Live" is the batch loader's own
> reach: inbox ROOT or `processing/cycle-*/`; quarantine/consumed/processed/rejected/retry
> are not. Source incidents: cycles 1487, 1497. Sibling: `carryover-lane-retirement-verifiableby`
> (third dispatch source needing the same retirement check).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ghost-scope-refused | Dead binding → nil, logged, released | 8/10 | `go test -tags acs -run TestC1507_004_...` |
| live-item-adopted | Live pending item → adopted, binding intact | 7/10 | `go test -tags acs -run TestC1507_005_...` |
| in-flight-live | Claimed item counts as live | 7/10 | `go test -tags acs -run TestC1507_006_...` |
| g1-untouched | Stamped claim still wins | 6/10 | `go test -tags acs -run TestC1507_007_...` |
