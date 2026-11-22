# Build Explanation — Cycle 2600

## Build Binding
- Cycle: 2600
- Base SHA: 253928c91d83c5c9f011bc1e600d05e13853447b

## Summary
The `record-pr` command now verifies that the fetched pull ref matches the PR head OID. When the pull ref lags, `Train.Record` polls within a policy bound. If the ref stays stale, `Train.Record` refuses and names both short SHAs.

## Rationale
GitHub updates `refs/pull/N/head` asynchronously after a branch push. Previously, `Train.Record` trusted the first fetch and wrote a stale commit list. Later, `integration batch` refused the PR because its tip did not match. Checking the ref tip against `gh pr view --json headRefOid` fixes the race.

## Changed Areas
- `go/internal/policy/fleet_integration.go` — adds `DefaultPullRefWaitS`, `FleetIntegrationPolicy.PullRefWaitS`, and `Policy.PullRefWait()`. Zero disables waiting, and negative values fail closed.
- `go/internal/integration/train.go` — adds `PRHost.HeadOID`, `Train.PullRefWait`, `fetchPRHead`, `waitPullRef`, and `sleepContext`. `Train.Record` compares `refs/evolve/pr/N` with the head OID, retries inside the bound, and returns `*Refusal`.
- `go/internal/integration/ghci.go` — implements `GhHost.HeadOID` through `gh pr view <pr> --json headRefOid`. It returns an error when `headRefOid` is empty.
- `go/cmd/evolve/cmd_integration_batch.go` — adds `pullRefWait` and sets `Train.PullRefWait` in `integrationEnv.train` from `.evolve/policy.json`. Both `record-pr` and `batch` receive the policy bound.

## Design Decisions
`Train.Record` reads the PR head OID through `PRHost.HeadOID` and retries `+refs/pull/N/head:refs/evolve/pr/N`. This keeps the train anchored to the pull ref that GitHub tests and merges. We rejected fetching `refs/heads/<branch>` directly because the batch step composes from `refs/pull/N/head`. Storing the default wait in Go policy data follows the policy-as-data convention.

## Verification
Unit tests in `internal/policy` and `internal/integration` pass after the final edit. CLI integration tests in `cmd/evolve` and all four predicates in `acs/cycle2600` pass. `evolve acs suite --cycle 2600` reports `PASS` with zero red predicates.

## Compatibility
`Train.PullRefWait` defaults to zero on direct struct literals, which disables waiting. `integrationEnv.train` sets it to 5 seconds by default from policy. `GhHost.HeadOID` requires a non-empty `headRefOid` from `gh pr view`, while `fakeHost` permits empty values for unit tests.

## Limitations
Only `Train.Record` polls `refs/pull/N/head` against the head OID. `Train.Plan` and `Train.Settle` still use `Train.sync` because a recorded binding already pins the tip SHA.
