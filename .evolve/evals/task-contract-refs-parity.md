---
score_cap:
  - criterion: "taskItemRefs' id set and the rendered Task Contract equal ContractTaskIDs for every fixture of the shared parity table (pin, decision, deferral, empty, stale scope, padded ids)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTaskItemRefs_ContractTaskIDsParity$' ./internal/core"
  - criterion: "The Task Contract dispatched by the real RunCycle to tdd/build/audit names the committed set, never a stale request-context fleet scope"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1717_001_' ./acs/cycle1717/"
  - criterion: "ctx fleet_scope_paths only places a committed member's record (pair wins over the resolver); it never adds or removes a member"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestTaskItemRefs_ScopePathsResolvePathsNeverMembership$' ./internal/core"
  - criterion: "taskItemRefs derives membership through committedset — its call graph reaches committedset and none of BoundTaskIDs / LaneScopeIDs / deferredTaskIDs, and deferredTaskIDs is gone"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1717_003_' ./acs/cycle1717/"
  - criterion: "The Task Contract block renders byte-identically for a lane pin, a decision-only cycle and a deferral"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral$' ./internal/core"
  - criterion: "go vet is clean and the task-contract test family is race-free"
    max_if_missing: 5
    evidence: "cd go && go vet ./internal/core ./internal/committedset && go test -race -count=1 -run 'TaskContract|TaskItemRefs|ContractTaskIDs|LaneScopeIDs' ./internal/core"
---

# Eval: taskItemRefs derives the committed set from committedset

> Pins `task-contract-refs-parity` (inbox 2026-09-11, techdebt, weight 0.5):
> `core.ContractTaskIDs` delegates to `internal/committedset`, but its documented
> parity partner `core.taskItemRefs` re-derived the same ids inline from
> `ctx["fleet_scope_paths"]` / `ctx["fleet_scope"]` / `BoundTaskIDs` /
> `LaneScopeIDs` / `deferredTaskIDs`. Cycle 1717's RED run showed the two
> diverging on 11 of 19 shared fixtures on the unmodified tree, including a
> shape production reaches: a request-context scope with no lane pin while
> triage commits to a different id. There the Task Contract handed tdd, build
> and audit the stale id while the TDD->Build scope gate graded against the
> decision. The inbox item's claim that they "agree today" was false for the
> pre-existing `TestTaskItemRefs_DeferredScopeIsNotMandatory` fixture itself.
> Source incident: cycle 1717 bug-reproduction (`repro/task_contract_repro_test.go`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parity-table | refs == ContractTaskIDs for every shared fixture | 8/10 | `go test -run '^TestTaskItemRefs_ContractTaskIDsParity$' ./internal/core` |
| dispatch-reachability | RunCycle-dispatched contract names the committed set | 8/10 | `go test -tags acs -run '^TestC1717_001_' ./acs/cycle1717/` |
| paths-not-membership | fleet_scope_paths places records only | 7/10 | `go test -run '^TestTaskItemRefs_ScopePathsResolvePathsNeverMembership$' ./internal/core` |
| single-projection | membership reached through committedset only | 6/10 | `go test -tags acs -run '^TestC1717_003_' ./acs/cycle1717/` |
| byte-identical-render | pin / decision / deferral blocks unchanged | 7/10 | `go test -run '^TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral$' ./internal/core` |
| hygiene | vet clean, race-free task-contract family | 5/10 | `go vet …; go test -race -run 'TaskContract\|TaskItemRefs\|…' ./internal/core` |
