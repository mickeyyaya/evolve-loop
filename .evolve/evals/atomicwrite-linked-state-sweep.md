---
score_cap:
  - criterion: "storage.WriteState writes THROUGH a symlinked .evolve/state.json (absolute, relative, dangling): the link survives and the canonical file carries the write"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestWriteState_WritesThroughSymlinkedStatePath$' ./internal/adapters/storage | grep -q -- '--- PASS: TestWriteState_WritesThroughSymlinkedStatePath/dangling '"
  - criterion: "storage.UpdateState's locked lossless RMW writes THROUGH a symlinked .evolve/state.json (absolute, relative, dangling): the link survives and the canonical file carries the write"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestUpdateState_WritesThroughSymlinkedStatePath$' ./internal/adapters/storage | grep -q -- '--- PASS: TestUpdateState_WritesThroughSymlinkedStatePath/dangling '"
  - criterion: "storage.WriteCycleState writes THROUGH a symlinked .evolve/cycle-state.json (absolute, relative, dangling): the link survives and the canonical file carries the write"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestWriteCycleState_WritesThroughSymlinkedCycleStatePath$' ./internal/adapters/storage | grep -q -- '--- PASS: TestWriteCycleState_WritesThroughSymlinkedCycleStatePath/dangling '"
  - criterion: "The storage writers resolve their write target through the shared statemap.ResolveWriteTarget (reused, not re-implemented)"
    max_if_missing: 6
    evidence: "grep -l 'statemap.ResolveWriteTarget(' go/internal/adapters/storage/*.go | grep -qv '_test.go$'"
  - criterion: "AC3: the cycle-1694 build report records the atomicwrite-caller inventory (the three linked files, the measured production count, none targets a linked file, callers not migrated)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1694_009_BuildReportRecordsTheAtomicwriteCallerInventory$' ./acs/cycle1694"
  - criterion: "The storage package carries no regression (existing ioHooks seam, checkpoint splice and run.json mirror tests stay green)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/adapters/storage"
---

# Eval: storage state writers resolve worktree symlinks before tmp+rename

> Inbox item `atomicwrite-linked-state-sweep` (cycle 1694), step (3) of
> `statejson-latent-unresolved-writers`. `core/worktree.go` linkGuardDeps
> symlinks three state files into every cycle worktree: `state.json`,
> `ledger.jsonl` and `cycle-state.json`. `adapters/storage` `WriteState`,
> `WriteCycleState` and `UpdateState` all end in the private `writeJSONAtomic`.
> That function tmp+renames onto the UNRESOLVED path, and a rename over a
> symlink REPLACES the link with a regular file. That is the cycle-999 sever,
> and after it every later write strands in a detached copy. Cycle 1690 fixed the
> statemap, SealCycle and failurelog writers. This eval pins the last three,
> which the 2026-09-26 console premise audit found are latent today. Source
> incidents: cycle-999/1000; the cycle-1690 audit finding M1 carried this step
> forward.
>
> The durable tests are also proven RED on the pre-fix code by the cycle-1694
> ACS predicate `TestC1694_007_DurableTestsAreRedWithoutTheResolver`. It rewrites
> every `statemap.ResolveWriteTarget` reference in the storage package to an
> identity function via `go test -overlay` and requires every link shape to fail.
>
> Audit round 1 (cycle 1694) FAILed on AC3 (M1): the build report never recorded
> the atomicwrite-caller inventory. The AC3 grader runs the cycle-1694 ACS
> predicate `TestC1694_009`. It measures the inventory at base `4a210334` (53
> production call lines) and requires the report to record it. The predicate
> skips once the run workspace is archived.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| write-through (state.json) | WriteState keeps the link and lands on canonical | 9/10 | `go test -run '^TestWriteState_WritesThroughSymlinkedStatePath$' ./internal/adapters/storage` |
| write-through (state.json RMW) | UpdateState keeps the link and lands on canonical | 9/10 | `go test -run '^TestUpdateState_WritesThroughSymlinkedStatePath$' ./internal/adapters/storage` |
| write-through (cycle-state.json) | WriteCycleState keeps the link and lands on canonical | 9/10 | `go test -run '^TestWriteCycleState_WritesThroughSymlinkedCycleStatePath$' ./internal/adapters/storage` |
| reuse | the shared resolver is called, not re-implemented | 6/10 | a production storage file calls `statemap.ResolveWriteTarget(` |
| inventory recorded (AC3) | build-report.md records the atomicwrite-caller inventory, callers not migrated | 6/10 | `go test -tags acs -run '^TestC1694_009_' ./acs/cycle1694` |
| no-regression | storage suite green | 5/10 | `go test ./internal/adapters/storage` |
