# Build Explanation — Cycle 2414

## Build Binding
- Cycle: 2414
- Base SHA: 6ab831919ed81c1e40da9a0f2b1d816b9081f4e3

## Summary
This build implements `EnforceActor`, `WritePolicy` write-once deliverable protection, `SyncToOrigin`, `PromoteLanded`, and `PrefixCheck` removal for fleet integration at enforce.

## Rationale
`EnforceActor` implements the existing `Actor` interface so `Driver` supports `enforce` without changing its core state machine. `WritePolicy` prevents duplicate enforce ships from overwriting delivered records while shadow keeps `ReplaceExisting`.

## Changed Areas
- `go/internal/integration/enforce_actor.go` — adds `EnforceActor` (`Resume`, `Flush`, `WaveEnd`) and its port interfaces (`PlaneSync`, `LandingFeed`, `Requeuer`, `BriefStore`, `Promoter`).
- `go/internal/integration/sync.go` — adds `SyncToOrigin` and `SyncOutcome` (`current`, `moved`, `ahead`, `diverged`, `blocked`) without `--autostash`.
- `go/internal/integration/driver.go` — updates `StageActors` to register `policy.FleetIntegrationEnforce` when non-nil.
- `go/internal/integration/shadow_actor.go` — reuses shared pool-ref cleanup helpers (`dropPoolRefs`, `deleteBranchRef`).
- `go/internal/integration/deliverable.go` — adds `WritePolicy` (`ReplaceExisting`, `WriteOnce`), `ErrRecordExists`, and `WriteRecordWith`.
- `go/internal/integration/batchtest.go` — moves input validation (`checkBatchInputs`) out of deleted `prefix.go`.
- `go/internal/integration/prefix.go` — deletes `PrefixCheck` and its dead L1 prefix-check path.
- `go/internal/phases/ship/deliverable.go` — selects `WriteOnce` when `deliverEnforced()` is true.
- `go/internal/phases/ship/postship.go` — renames and exports `promoteLandedCycle` as `PromoteLanded` for future `Promoter` callers.
- `go/cmd/evolve/cmd_loop_driver.go` — updates `StageActors` call site with `nil` enforce actor.

## Design Decisions
`WriteOnce` links the staged temp file to the target path and removes the temp file. An existing record returns `ErrRecordExists` atomically with zero temp-file leaks. `SyncToOrigin` checks ancestry before `merge --ff-only` and returns `SyncBlocked` on any merge failure without stashing operator edits.

## Verification
All 16 cycle-2414 ACS predicates (`go test -count=1 -tags acs ./acs/cycle2414/...`), `internal/integration`, and targeted `internal/phases/ship` tests pass after the edits.

## Compatibility
Shadow delivery keeps `ReplaceExisting` through `WriteRecord`. `StageActors(shadow, nil)` leaves `enforce` unregistered until console wiring enables it.

## Limitations
Protected control-plane surfaces (`consoleroute.go`, `inboxmover`, and `core` resume/rollout wiring) are excluded from this lane and deferred per triage. This build also omits the F12 bookkeeping flush, mid-wave sync, rollout gate, and `Promoter` adapter.
