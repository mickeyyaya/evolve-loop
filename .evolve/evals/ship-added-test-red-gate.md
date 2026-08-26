---
score_cap:
  - criterion: "A newly added test that fails in the lane worktree blocks ship with CodeRepoContractGate, naming the failing test"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_NewlyAddedFailingTestBlocksShip$' ./internal/phases/ship"
  - criterion: "Selection is bounded to newly added Go _test.go files — modified tests, non-test files, and empty diffs never trip the gate"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles$' ./internal/phases/ship"
  - criterion: "The four pre-existing repo-contract gate behaviours are unweakened by this addition"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_(OffSkips|EnforceGreenPasses|EnforceRedFailsWithDedicatedCode|UnknownStageFailsTowardEnforce)$' ./internal/phases/ship"
---

# Eval: Block newly added red tests at ship time

> Pins the fix for the red-first-deliverable-reds-main incident: three lanes
> allowed an intentionally red reproduction test to reach main because the
> ship-time repo-contract scanner (`go/internal/phases/ship/repocontract.go`)
> only runs four FIXED guard packages (`phasespec`, `profiles`,
> `phasecoherence`, `routingtest`). Any new test file added by the shipping
> diff itself was structurally invisible to it — there was no consumer that
> ever looked at what the diff added.
>
> The fix derives the newly added, in-module Go `_test.go` paths from the
> shipping tree (staged/committed relative to the lane's baseline) and
> executes ONLY that bounded set through the existing classification pipeline,
> reusing the established `CodeRepoContractGate` structured error rather than
> inventing a parallel code path. The selection MUST be bounded in both
> directions: it must catch a genuinely added red test, and it must NOT widen
> to modified tests (the fixed four packages already own modified-test
> regressions in their own scope) or non-test files, or misfire on an empty
> shipping diff — an over-eager detector is exactly as dangerous as an
> under-eager one, since it would false-RED an honest ship.
>
> Source incident: live inbox `red-first-deliverable-reds-main`; cycle-1550's
> ordering fix is a SEPARATE, already-in-progress console fix and is out of
> scope here — this is the independently-valuable delivery-boundary floor that
> protects any future route which adds a red test after Build.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| added-red-blocks | A newly added failing test blocks ship, naming it | 9/10 | `go test -run '^TestRepoContractGate_NewlyAddedFailingTestBlocksShip$'` |
| bounded-selection | Modified/non-test/empty diffs never trip the gate | 8/10 | `go test -run '^TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles$'` |
| anti-weakening | The four pre-existing gate behaviours survive unmodified | 7/10 | `go test -run '^TestRepoContractGate_(OffSkips|EnforceGreenPasses|EnforceRedFailsWithDedicatedCode|UnknownStageFailsTowardEnforce)$'` |
