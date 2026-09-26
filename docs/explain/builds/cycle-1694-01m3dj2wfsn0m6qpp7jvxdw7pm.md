# Build Explanation — Cycle 1694

## Build Binding
- Cycle: 1694
- Base SHA: e95b4fc220d78d96de8598f88ca1664a8238a327

## Summary
The storage adapter's three state writers — `WriteState`, `UpdateState` and `WriteCycleState` — now resolve a symlinked `.evolve/state.json` or `.evolve/cycle-state.json` to its final target before the tmp+rename. The rename lands on the canonical file and the worktree link survives. This closes step 3 of `statejson-latent-unresolved-writers`: the last writers still exposed to the cycle-999 sever.

## Rationale
`core/worktree.go` `linkGuardDeps` symlinks each cycle worktree's `state.json` and `cycle-state.json` at their canonical files. A rename onto the unresolved path replaces the link with a regular file, so every later write strands in a detached copy. Cycle 1690 added `statemap.ResolveWriteTarget` and moved the statemap, `SealCycle` and failurelog writers onto it. Reusing that resolver is the smallest change and keeps one bounded, dangling-tolerant symlink walker in the tree. A second implementation inside storage was rejected because it would duplicate the resolver and could drift from it.

## Changed Areas
- `go/internal/adapters/storage/statejson.go` — `WriteState` and `WriteCycleState` now resolve their target path with `statemap.ResolveWriteTarget`. `WriteCycleState` does its sidecar lock, checkpoint read and rename on the resolved path. The run.json mirror is unchanged.
- `go/internal/adapters/storage/updatestate.go` — `UpdateState` resolves the path before it takes the flock. The `<state.json>.lock` sidecar it holds is then the canonical one that `statemap.UpdateStateMap` also contends on, and the RMW rename keeps the link intact.
- `.evolve/inbox/2026-09-26T00-00-00Z-atomicwrite-linked-state-sweep.json` — removed from the live inbox. The lane's ship step consumes the item it delivers, so the pending record is retired rather than left to be re-dispatched after it lands.
- `.evolve/inbox/consumed/2026-09-26T00-00-00Z-atomicwrite-linked-state-sweep.json` — the same record, moved here with a `consumed` stamp (`via: ship`) and its keys re-serialised in sorted order. The acceptance text is unchanged, so the audit trail keeps the exact criteria this build was graded against.
- `go/internal/adapters/storage/statejson_symlink_test.go` — adds durable regression tests for each writer across three link shapes: absolute, relative, and dangling until first write. Each test asserts that the link is still intact and that the canonical file received the bytes.

## Design Decisions
Only the write target changes; storage and statemap stay separate paths. Storage keeps its typed replace (`WriteState`, with no CAS floor) and its own `stateRevision` counter (`UpdateState`). It never routes through `statemap.WriteStateMap` or `UpdateStateMap`. Locks move to the resolved path because the resolver's contract says every writer of a possibly-linked file must lock the resolved path. That way a worktree writer and a canonical-path writer contend on the same sidecar. The storage package now imports statemap, which is a leaf that imports only flock, so this adds no import cycle.

## Verification
`go test -count=1 ./internal/adapters/storage` passes, including the existing ioHooks, checkpoint-splice and run.json mirror tests. All 9 new durable subtests fail on the base code because the link is replaced by a regular file, and pass with the fix. The cycle-1694 ACS predicates report 9 PASS, 2 SKIP and 0 FAIL. Predicates 010 and 011 skip themselves because they are pinned to the pre-rebase base `4a210334`, while HEAD is now `e95b4fc2`. Predicate 007 checks the tests again against the base behaviour by swapping the resolver for an identity function through `go test -overlay`, and requires every link shape to fail.

## Compatibility
When state files are regular files, as in the main tree, the resolver returns the path unchanged, so behaviour and lock paths are identical to before. No public API or on-disk format changed.

## Limitations
`checkpoint.ApplyToStateFile` still locks and renames on the path it is given. It has no production caller today; only tests call it. If a future caller passes a worktree's linked `cycle-state.json`, the write would sever that link and lock a different sidecar than `WriteCycleState`. That package was outside this lane's scope and is recorded as a follow-up.
