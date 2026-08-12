---
score_cap:
  - criterion: "a second CONSECUTIVE git-add refusal of the SAME pathspec in the same workspace is classified deterministic (ShipClassPrecondition), not transient"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestStageRefusal_SecondSamePathspecIsDeterministic$' ./internal/phases/ship"
  - criterion: "the two-strikes rule matches the PATHSPEC and is workspace-scoped — a first strike, a different pathspec, a peer lane, or a workspace-less run all stay transient"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestStageRefusal_(FirstStrikeStaysTransient|DifferentPathspecStaysTransient|SeparateWorkspacesDoNotShareStrikes|NoWorkspaceStaysTransient)$' ./internal/phases/ship"
---

# Eval: identical stage refusals classify deterministic, not transient

> Pins the retry-budget boundary at the ship staging seam
> (`stageExplicitPaths`, go/internal/phases/ship/gitops.go). Every `git add`
> refusal was stamped `core.ShipClassTransient`, so the failure floor kept
> re-dispatching a refusal that could never succeed in place. Live incident:
> cycle-1365 refused the SAME `.evolve/evals` pathspec twice — its continuation
> worktree base predated the .gitignore carve-out — and burned the entire retry
> budget on an unwinnable add before the cycle died.
> Source incident: cycle-1365, landed as cycle-1440 inbox item
> `deterministic-stage-refusal-router`.
>
> The negative criterion carries the HIGHER cap: a rule that merely counts
> refusals (instead of matching the pathspec) would satisfy the positive case
> while deleting the legitimate retry for an unrelated flaky add, and a
> lane-global strike memory would let one fleet lane deterministically block a
> peer's first attempt.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| two-strikes-positive | second identical refusal ⇒ precondition class | 7/10 | `go test -run '^TestStageRefusal_SecondSamePathspecIsDeterministic$' ./internal/phases/ship` |
| retry-preservation | first strike / different pathspec / peer lane / no workspace stay transient | 8/10 | `go test -run '^TestStageRefusal_(FirstStrikeStaysTransient\|DifferentPathspecStaysTransient\|SeparateWorkspacesDoNotShareStrikes\|NoWorkspaceStaysTransient)$' ./internal/phases/ship` |
