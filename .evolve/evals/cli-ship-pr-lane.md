---
score_cap:
  - criterion: "evolve ship --class manual --pr creates the PR through gh and prints its number"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -v -run '^TestRunShipCmd_Pr_CreatesThePullRequestAndPrintsItsNumber$' ./cmd/evolve/ | grep -q -- '--- PASS: TestRunShipCmd_Pr_CreatesThePullRequestAndPrintsItsNumber'"
  - criterion: "a branch that already has an open PR reports it and creates no second one"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestRunShipCmd_Pr_ReportsAnOpenPullRequestInsteadOfCreatingASecond$' ./cmd/evolve/ | grep -q -- '--- PASS: TestRunShipCmd_Pr_ReportsAnOpenPullRequestInsteadOfCreatingASecond'"
  - criterion: "--pr on main exits 10 before ship or gh run"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestRunShipCmd_Pr_OnMainExits10BeforeShipOrGhRun$' ./cmd/evolve/ | grep -q -- '--- PASS: TestRunShipCmd_Pr_OnMainExits10BeforeShipOrGhRun'"
  - criterion: "a gh failure exits 2 and leaves the push result in place"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestRunShipCmd_Pr_AGhFailureExits2AfterShipKeptTheCommitPushed$' ./cmd/evolve/ | grep -q -- '--- PASS: TestRunShipCmd_Pr_AGhFailureExits2AfterShipKeptTheCommitPushed'"
---

# Eval: evolve ship --pr opens the pull request of a pushed branch

> Pins the `--pr` / `--pr-body-file` flags of `evolve ship --class manual`.
> The command pushed a branch and left the PR to raw `gh` calls in the /commit
> and /evo:publish scripts. Source: inbox card `cli-ship-pr-lane`, cycle 1992.
> The tests drive `runShipCmd` with a fake `gh` first on PATH; ship itself is
> the existing `shipRunFn` seam, so a PR failure never undoes a push.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| create | the PR is created and its number printed | 4/10 | `TestRunShipCmd_Pr_CreatesThePullRequestAndPrintsItsNumber` |
| idempotent | an open PR is reported, no second create | 5/10 | `TestRunShipCmd_Pr_ReportsAnOpenPullRequestInsteadOfCreatingASecond` |
| main-guard | `--pr` on main exits 10 | 5/10 | `TestRunShipCmd_Pr_OnMainExits10BeforeShipOrGhRun` |
| gh-failure | exit 2, push kept | 6/10 | `TestRunShipCmd_Pr_AGhFailureExits2AfterShipKeptTheCommitPushed` |
