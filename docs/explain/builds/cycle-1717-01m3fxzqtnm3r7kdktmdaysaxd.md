# Build Explanation — Cycle 1717

## Build Binding
- Cycle: 1717
- Base SHA: 447c0744ed0ede94cca4d5d36c01bd2d2a08e597

## Summary
`core.taskItemRefs` now takes its member ids from `ContractTaskIDs`, which is
the `committedset` projection. That projection is the lane pin if one exists,
otherwise triage's `top_n`, minus any deferrals. Before this change the
function rebuilt the same set inline from `ctx["fleet_scope_paths"]`,
`ctx["fleet_scope"]`, `BoundTaskIDs`, `LaneScopeIDs` and a duplicate deferral
reader, `deferredTaskIDs`. The dispatch context now only supplies file paths.
A `fleet_scope_paths` `id=path` pair points to a committed member's inbox record
and wins over the scope-path resolver. A pair cannot add or remove a member. The
duplicate readers `deferredTaskIDs`, `scopedTaskItemRefs` and the core-local
`splitCSV` are deleted.

## Rationale
`ContractTaskIDs` is documented as the parity partner of `taskItemRefs`. The
TDD->Build scope gate grades against it. The two functions still disagreed on 11
of the 19 shared fixtures. That included a shape production reaches: a
request-context fleet scope with no lane pin while triage commits to a different
id. In that case tdd, build and audit got the stale id's acceptance, but the
gate graded the decision. Taking membership from the one projection removes the
disagreement by construction. It does not add a rule to reconcile two
derivations.

## Changed Areas
- `go/internal/core/task_contract.go` — `taskItemRefs` now reads `ContractTaskIDs` for membership and adds a small `scopePathPairs` parser for paths. The inline membership readers (`scopedTaskItemRefs`, `deferredTaskIDs`, `splitCSV`) and the unused `slices` import are deleted.
- `go/internal/core/task_contract_test.go` — the TDD phase added the shared parity table, the paths-never-membership test and the byte-identical render test. Existing fixtures now write the on-disk binding that production writes next to every fleet scope.
- `go/acs/cycle1717/predicates_test.go` — the TDD phase's ACS predicates. They cover the RunCycle dispatch reachability, the parity binding, the call-graph check that membership reaches `committedset` and none of the legacy readers, and vet plus `-race`.
- `.evolve/evals/task-contract-refs-parity.md` — the task's eval and its score-cap graders.

## Design Decisions
Membership goes through `ContractTaskIDs` rather than a direct call to
`committedset.Committed`. That keeps one kernel-side name for "the committed
set", the same one the scope gate and the parity test read. The call graph
still reaches `committedset`. Paths come first from a disclosed pair, then from
the resolver. A member that neither source can place is rendered unresolved,
never dropped. That keeps the block's "loud gap" contract. A malformed pair now
places nothing and does not become a phantom member. Its id renders only if the
binding commits to it.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1717/`: all four predicates pass.
- `go test -race` on the task-contract, taskItemRefs, ContractTaskIDs and LaneScopeIDs test family passes.
- `go vet ./internal/core ./internal/committedset` is clean.
- `go test -count=1 ./internal/core/...` passes in all 8 packages.

## Compatibility
The rendered block is byte-identical for a lane pin, a decision-only cycle and a
deferral (pinned by `TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral`).
The behavior change is deliberate. A context fleet scope with no on-disk binding
(no lane pin and no triage decision) no longer produces a Task Contract. It
binds nothing, which matches `ContractTaskIDs`. The Task Recall seeding in
`task_recall.go` also goes through `taskItemRefs`, so it now follows the same
committed set.

## Limitations
`BoundTaskIDs` and `LaneScopeIDs` stay in place for their other consumers
(failure digest, solution floor, ship). Only the Task Contract's derivation
moved onto `committedset`.
