---
score_cap:
  - criterion: "A cycle drain emits one inbox.released signal per item it moves out of processing/cycle-N/"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestReleaseOne_EmitsInboxReleased$' ./internal/inboxmover/lifecycle"
  - criterion: "A stale-claim sweep warns with INBOX_CLAIM_STALE for a sealed cycle that still holds a claim"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestStaleClaimSweep_SealedCycleStillInProcessingWarns$' ./internal/inboxmover"
  - criterion: "A failed, system-halted cycle releases its claims and signals each release"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestApplyFailure_SystemHaltedCycleReleasesClaims$' ./internal/inboxmover"
---

# Eval: claim lifecycle signals and lane-exit release

> Pins the claim-release signal contract of the inbox mover. In wave 182 the claims of failed cycles 2547, 2548 and 2551 stayed in processing/ until a console release --stale: 169 inbox.claimed events existed and no inbox.released event. The cycle drain (`Mover.Release` -> `releaseOne`) moved items without a signal, and nothing warned for a sealed cycle that still held a claim. Source incident: cycle 2566 inbox item claim-lifecycle-signals-and-lane-exit-release.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| drain-release-signal | one inbox.released per drained item | 6/10 | `go test -run TestReleaseOne_EmitsInboxReleased ./internal/inboxmover/lifecycle` |
| sealed-claim-warn | INBOX_CLAIM_STALE WARN for a sealed cycle's claim | 6/10 | `go test -run TestStaleClaimSweep_SealedCycleStillInProcessingWarns ./internal/inboxmover` |
| halted-cycle-release | system-halted failure releases and signals | 7/10 | `go test -run TestApplyFailure_SystemHaltedCycleReleasesClaims ./internal/inboxmover` |
