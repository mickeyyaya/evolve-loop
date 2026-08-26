---
score_cap:
  - criterion: "A real newly added failing test drives Phase.runNative to stop before any git/ship action, with the structured repo-contract error"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestPhaseRunNative_NewlyAddedFailingTestPreventsRun$'"
  - criterion: "A newly added t.Skip-ped test does not block the production runNative path"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRunNative_AddedSkippedTestDoesNotBlockShip$'"
  - criterion: "runNative threads the run workspace into the gate so the scan log lands in the run dir"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRunNative_RepoContractGateReceivesRunWorkspace$'"
---

# Eval: Prove the new-test floor fires on the production ship path

> The cheapest fake for the added-test gate is a test that calls the scanner
> helper directly: it passes even if `runNative` never reaches the helper, or
> reaches it after `Run()` has already pushed. Cycles 1563 and 1564 both failed
> audit on exactly that dead-plumbing class. This eval pins the reachability
> proof instead — a real temporary worktree with a real staged failing test,
> driven through `Phase.runNative` (`go/internal/phases/ship/ship.go`), must
> return the structured repo-contract error BEFORE `Run(ctx, opts)`, and must
> leave its scan log in the run workspace. The negative half is equally
> load-bearing: a clean or honestly-skipped worktree must sail through the same
> production path. Source incidents: cycle-1563 / cycle-1564 dead-plumbing audit
> findings.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| production-caller-blocks | `runNative` stops before git/ship on a real added red test | 9/10 | `go test -run '^TestPhaseRunNative_NewlyAddedFailingTestPreventsRun$'` |
| production-caller-passes | An honest `t.Skip` reproducer is not blocked on the same path | 7/10 | `go test -run '^TestRunNative_AddedSkippedTestDoesNotBlockShip$'` |
| workspace-threaded | The run workspace reaches the gate, so the scan log is auditable | 6/10 | `go test -run '^TestRunNative_RepoContractGateReceivesRunWorkspace$'` |
