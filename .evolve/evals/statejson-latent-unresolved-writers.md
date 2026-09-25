---
score_cap:
  - criterion: "statemap exports ResolveWriteTarget with the bounded, dangling-tolerant chain semantics (relative and absolute hops, final target returned, dangling tail returned not errored)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestResolveWriteTarget$' ./internal/adapters/statemap | grep -q -- '--- PASS: TestResolveWriteTarget'"
  - criterion: "core.SealCycle takes the state lock on the RESOLVED canonical path (the <canonical>.lock sidecar every canonical-path writer contends on), never beside a worktree link"
    max_if_missing: 8
    evidence: "cd go && env -u EVOLVE_CYCLE_STATE_FILE go test -count=1 -v -run '^TestSealCycle_SymlinkedStateLocksCanonicalTarget$' ./internal/core | grep -q -- '--- PASS: TestSealCycle_SymlinkedStateLocksCanonicalTarget'"
  - criterion: "Every failurelog state writer (Record, PruneExpired, PruneByClassification, PruneExpiredCarryoverTodos, BackfillLegacyCarryoverExpiry, IncrementCarryoverUnpicked) writes THROUGH a symlinked state.json: the link survives and the canonical file carries the write"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestStateWriters_PreserveSymlinkedStatePath$' ./internal/failurelog | grep -q -- '--- PASS: TestStateWriters_PreserveSymlinkedStatePath'"
  - criterion: "failurelog.Record keeps its never-auto-create contract: an absent (or dangling-linked) state.json returns ErrStateMissing and writes nothing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestRecord_StateMissing$' ./internal/failurelog | grep -q -- '--- PASS: TestRecord_StateMissing'"
  - criterion: "ResolveWriteTarget is a real, documented export of the apicover-enrolled statemap package and a statemap test names it"
    max_if_missing: 6
    evidence: "cd go && go doc ./internal/adapters/statemap ResolveWriteTarget | grep -q 'func ResolveWriteTarget(path string) string' && grep -lq 'ResolveWriteTarget' internal/adapters/statemap/*_test.go"
  - criterion: "The three touched packages carry no regression (statemap, failurelog, and core's seal/reset family)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/adapters/statemap ./internal/failurelog && env -u EVOLVE_CYCLE_STATE_FILE go test -count=1 -run '^Test(SealCycle|AutosealStaleMarker|MarkerShouldAutoseal)' ./internal/core"
  - criterion: "The deferred how_to_apply step (3), the sweep of other atomicwrite users writing linkable .evolve/ state, is filed as its own tracked inbox record that survives the lane item's PASS consume, and the cycle-1690 explanation Limitations states the consume and cites that record by id"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1690_01[01]_' ./acs/cycle1690 | grep -cE '^--- PASS: TestC1690_01[01]_' | grep -qx 2"
---

# Eval: latent cycle-999-class state.json writers (unresolved lock path + raw tmp+rename)

> Pins the fix for inbox item `statejson-latent-unresolved-writers` (go-reviewer
> MEDIUM on the statemap CAS/symlink fix, 2026-07-21), landed in cycle 1690. A
> worktree's `.evolve/state.json` is an absolute symlink to the canonical host
> state file (`core/worktree.go` `linkGuardDeps`). `statemap.WriteStateMap` /
> `UpdateStateMap` already resolve that link before locking and before the
> tmp+rename; two writers did not. `core.SealCycle` locked the UNRESOLVED path,
> so a seal through a linked evolve dir contended on a sidecar no canonical-path
> writer takes (cross-tree lost update). Every `failurelog` state writer
> tmp+renamed the raw path, which REPLACES the link with a regular file, the
> cycle-999 sever, after which every later mutation strands in a detached copy.
> The fix exports `statemap.ResolveWriteTarget` and routes both through it.
> Source incidents: cycle-999/1000 (stranded writes through a severed
> worktree link); cycle 1690 (this pin). The cycle-scoped ACS predicates live
> in `go/acs/cycle1690/predicates_test.go`. This file is the permanent record.
>
> Correction vs. the scout's per-task draft: the draft required `failurelog.Record`
> to "succeed and materialize the file" through a dangling link. That contradicts
> Record's documented contract (preflight owns creating state.json) and the
> existing `TestRecord_StateMissing`. The contract-consistent behavior is pinned
> instead: a dangling link is left untouched and Record returns `ErrStateMissing`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| export-semantics | ResolveWriteTarget follows chains to the final target, tolerates a dangling tail | 7/10 | `go test -run '^TestResolveWriteTarget$' ./internal/adapters/statemap` |
| seal-lock-unification | SealCycle locks `<canonical>.lock`, not `<link>.lock` | 8/10 | `go test -run '^TestSealCycle_SymlinkedStateLocksCanonicalTarget$' ./internal/core` |
| failurelog-write-through | all six failurelog writers keep the link and land on canonical | 9/10 | `go test -run '^TestStateWriters_PreserveSymlinkedStatePath$' ./internal/failurelog` |
| no-auto-create | Record's ErrStateMissing contract survives the resolve step | 6/10 | `go test -run '^TestRecord_StateMissing$' ./internal/failurelog` |
| apicover | documented export named by a statemap test | 6/10 | `go doc … ResolveWriteTarget` + test-name grep |
| no-regression | statemap + failurelog + core seal/reset family green | 5/10 | package suites |
| deferred-step-tracked | step (3) has a tracked follow-up record that survives the PASS consume; Limitations states the consume and cites it | 6/10 | `go test -tags acs -run '^TestC1690_01[01]_' ./acs/cycle1690` |

Audit round 1 (cycle 1690, M1) added the `deferred-step-tracked` cap. The lane
item's how_to_apply step (3) was deferred in prose only, and the explanation
said it "stays open on the inbox record". A PASS landing consumes the whole lane
item (`ship/postship.go` `committedInboxIDs`, the cycle-1515 decomposition
shape), so step (3) would have vanished. The cap needs a tracked follow-up record
and a Limitations section that cites it. Both predicates still accept the record
after a later cycle consumes it (they look in `consumed/` too), so the cap stays
valid after the follow-up ships.

`env -u EVOLVE_CYCLE_STATE_FILE` guards the core invocations: a fleet
orchestrator `os.Setenv`s that key to the live lane's cycle-state
(`core/cyclerun.go`), and a SealCycle test inheriting it would seal and delete
the running cycle's state.
