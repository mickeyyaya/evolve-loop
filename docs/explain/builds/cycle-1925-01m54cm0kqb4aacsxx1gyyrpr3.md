# Build Explanation — Cycle 1925

## Build Binding
- Cycle: 1925
- Base SHA: 814bfe5ea84ab874785680ca94348595ea3b4dcc

## Summary
`evolve pr merge --update-branch` now waits for GitHub to report the new head after `gh pr update-branch`. It awaits CI on that new head, not on the old one.

## Rationale
Before this change, `upToDate` read the PR once at once after the update. GitHub had not moved `headRefOid` yet, so `awaitGreen` checked the old head and `behindBy` refused it. A train of PRs failed on the first pass (PR #916, 2026-10-17). A bounded poll is the smallest fix that removes the race.

## Changed Areas
- `go/cmd/evolve/cmd_pr.go` — `upToDate` calls the new `awaitNewHead`, which polls `view` at `m.poll` until the head is non-empty and differs from the old head, and refuses at the `m.wait` deadline.
- `go/cmd/evolve/cmd_pr_update_head_test.go` — the TDD tests for a lagging head, a two-PR train, and the timeout, empty-head and view-failure negatives.
- `go/acs/cycle1925/predicates_test.go` — the cycle predicates that run those tests and the existing PR merge tests.
- `.evolve/evals/pr-merge-update-branch-reads-stale-head.md` — the eval for this bug.
- `docs/operations/runtime-reference.md` — the `evolve pr merge` entry now states the wait for the new head.

## Design Decisions
The head wait reuses the `--wait` deadline and the `ci_watch.poll_s` interval, so no new flag or policy key is added. The wait has its own deadline, separate from the CI wait. An empty head counts as no new head. A `gh` failure inside the wait exits 2, and a head that does not move refuses with exit 1. Rejected option: one fixed sleep before the second view. It cannot bound the GitHub lag and wastes time when the head moves fast.

## Verification
`go test -count=1 -run '^(TestPRMerger_|TestParsePRMergeArgs_|TestRunPR_)' ./cmd/evolve` passes, and so does the full `./cmd/evolve` package. The ACS suite for cycle 1925 is green.

## Compatibility
The CLI flags, the exit codes and the stdout format do not change. A PR that is not behind its base never enters the wait.

## Limitations
In the worst case, a PR that needs an update can wait twice `--wait`: once for the head and once for CI.
