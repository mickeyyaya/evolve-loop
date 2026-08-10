---
score_cap:
  - criterion: "A distinct CodeRepoContractInfra ship code exists with ShipClassPrecondition and does not alias CodeRepoContractGate"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCodeRepoContractInfra_Vocab$' ./internal/shiperr"
  - criterion: "A genuine go test failure stays CodeRepoContractGate and is NOT retried"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry$' ./internal/phases/ship"
  - criterion: "An unclassifiable first failure followed by a green retry lets the ship proceed, pack invoked exactly twice"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_TransientFailureRetriesOnceThenShips$' ./internal/phases/ship"
  - criterion: "Persistent unclassifiable failure returns the infra code after exactly two runs (no retry storm)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns$' ./internal/phases/ship"
  - criterion: "The four pre-existing repo-contract gate behaviours are preserved unmodified"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_(OffSkips|EnforceGreenPasses|EnforceRedFailsWithDedicatedCode|UnknownStageFailsTowardEnforce)$' ./internal/phases/ship"
---

# Eval: repo-contract gate infra-ambiguity classification

> Pins the exit-code classification contract of the ship-time repo-contract
> scanner pack (`go/internal/phases/ship/repocontract.go`). Before cycle-1409,
> `defaultRepoContractTest` returned `cmd.Run()`'s error verbatim and
> `runRepoContractGate` wrapped ANY non-nil error as `CodeRepoContractGate` — so
> a build-cache contention, module-fetch flake, or OOM-kill was indistinguishable
> from a genuinely red guard suite. That swallowed ambiguity produced a false RED
> that blocked three audit-green ships (cycles 1402/1403/1405); the preserved
> worktree snapshot `e0638346` re-ran 4/4 GREEN against the identical tree, and
> the clean baseline `cba017c5` was 4/4 GREEN too. The fix classifies
> `go test -json` events: a real test/compile failure stays a contract RED
> (unchanged, un-retried), anything else is retried once and — if still
> unclassifiable — returned as a distinct infra-classed code so the router can
> tell "fix your code" from "safe to re-dispatch".
>
> Source incident: cycles 1402/1403/1405 ship|gate-block storm; inbox item
> `repocontract-gate-false-red-swallowed-diag` FIX SHAPE (a).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| distinct-vocabulary | New infra code exists, non-aliasing, precondition class | 7/10 | `go test -run TestCodeRepoContractInfra_Vocab ./internal/shiperr` |
| real-red-unchanged | Genuine failure → `REPO_CONTRACT_GATE`, one run, no retry | 8/10 | `go test -run TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry ./internal/phases/ship` |
| transient-recovers | Flake + green retry → ship proceeds, exactly two runs | 9/10 | `go test -run TestRepoContractGate_TransientFailureRetriesOnceThenShips ./internal/phases/ship` |
| persistent-is-infra | Ambiguous twice → infra code, exactly two runs (negative case) | 8/10 | `go test -run TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns ./internal/phases/ship` |
| anti-weakening | Pre-existing off/green/red/unknown-stage behaviours survive | 6/10 | `go test -run 'TestRepoContractGate_(OffSkips\|EnforceGreenPasses\|EnforceRedFailsWithDedicatedCode\|UnknownStageFailsTowardEnforce)' ./internal/phases/ship` |
