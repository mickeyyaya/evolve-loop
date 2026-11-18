# Build Explanation — Cycle 2466

## Build Binding
- Cycle: 2466
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
The inbox verb group moved from `go/cmd/evolve` to `go/internal/cli/inboxcmd`. `main` keeps one shim file that builds a `Deps` value and calls `inboxcmd.Run`.

## Rationale
Step 1 of the split plan asks for the inbox group first, because it has the best ratio of relief to risk. A lane that edits an inbox verb no longer holds the admission cell of `package main`.

## Changed Areas
- `go/internal/cli/inboxcmd/cmd_inbox.go` — the verb table and `Run`, which stores `Deps` and dispatches the verb.
- `go/internal/cli/inboxcmd/deps.go` — the `Deps` value with six host functions, and the exit codes.
- `go/internal/cli/inboxcmd/doc.go` — the package doc.
- `go/internal/cli/inboxcmd/apicover_named_test.go` — names every export in a real assertion.
- `go/internal/cli/inboxcmd/host_test.go` — the test `Deps` and the shared test helpers of the package.
- `go/internal/cli/inboxcmd/cmd_inbox_failure_paths_test.go` — the lint, load-warning and quarantine failure-path tests that moved from `main`.
- `go/internal/cli/inboxcmd/inboxcmd_run_test.go` — the red test `TestInboxCmdRun` for the new package.
- `go/cmd/evolve/cmd_inbox.go` — the shim: `Deps` built from the functions of `main`, `runInbox` and `runInboxMover`.
- `go/cmd/evolve/cmdutil/projectroot.go` — `TakeFlagValue` and `TakeProjectRoot`, shared by `main` and the new package.
- `go/cmd/evolve/cmdutil/projectroot_test.go` — tests the two flag helpers.
- `go/cmd/evolve/cmd_signals.go` — calls `cmdutil.TakeProjectRoot`.
- `go/cmd/evolve/cmd_cli_routing_write.go` — calls `cmdutil.TakeFlagValue`.
- `go/cmd/evolve/cmd_loop_blockerbreaker.go` — calls the exported reconcile functions.
- `go/cmd/evolve/cmd_pr.go` — calls `inboxcmd.CheckInboxTrailerBytes`.
- `go/cmd/evolve/cmd_supply_split.go` — calls `inboxcmd.ReadInboxItemObject`.
- `go/cmd/evolve/cmd_supply_wiring.go` — calls `inboxcmd.SplitForbidden`.
- `go/cmd/evolve/inbox_fixtures_test.go` — the fixtures that tests in `main` still need.
- `go/cmd/evolve/cmd_inbox_live_lane_test.go` — the live-lane refusal test, which needs the real queue gate.
- `go/cmd/evolve/cmd_loop_tasklevel_test.go` — the task-level failure test, which tests a function of `main`.
- `go/cmd/evolve/cli_contract_cover_test.go` — the lint test moved to the new package.
- `go/cmd/evolve/cmd_backfill_failure_paths_test.go` — two tests moved to the new package.
- `go/cmd/evolve/cmd_status_root_test.go` — the inbox list test now goes through the shim.
- `go/.apicover-enforce` — enrolls the new package in the apicover gate.
- `go/internal/inboxtest/inboxtest.go` — the one home of the inbox test fixtures that `main` and `inboxcmd` both need. The move had left a second copy of each fixture in the two test packages.
- `go/internal/inboxtest/apicover_named_test.go` — names every export of the fixture package in a real assertion.
- `docs/architecture/packages/internal-inboxtest.md` — the page of the fixture package.
- `go/.test-dup-baseline` — lowered from 334 to 333. The fixture package removes the duplicate copies, and the ratchet fails on a count below its baseline.
- `go/cmd/evolve/cli_contract_cases_integration_test.go` — builds the origin/main fixture with `gittest` and `inboxtest`, because `originMainRoot` moved to `inboxcmd`.
- `docs/architecture/packages/internal-cli-inboxcmd.md` — the package page.
- `go/cmd/evolve/cmd_inbox_ackfingerprint.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_ackfingerprint.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_add.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_add.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_blocked_by.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_blocked_by.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_claim.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_claim.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_claims.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_claims.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_consume.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_consume.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_curate.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_curate.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_curator.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_curator.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_curator_apply.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_curator_apply.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_curator_follow.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_curator_follow.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_list.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_list.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_mover.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_mover.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_quarantine.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_quarantine.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_rank.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_rank.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_rank_print.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_rank_print.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_route_console.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_route_console.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_route_lane.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_route_lane.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_show.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_show.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_split.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_split.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.
- `go/cmd/evolve/cmd_inbox_trailercheck.go` — removed from `main`; the verb code moved unchanged in behavior to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_trailercheck.go` — the moved verb code; calls to host functions go through `Deps`, and exit codes use the local constants.

## Design Decisions
`Deps` holds only the functions that `main` alone can build. The flag helpers went to `cmdutil` because two packages use them. A verb reads `Deps` through a package-level atomic pointer that `Run` sets, which avoids a change to the signature of about 60 functions. The five functions that `main` calls became exports.

## Verification
The cycle predicates `TestC2466_001` to `TestC2466_005` run the new package test, the file check, the production binary, the apicover check and the dead-copy check. The moved white-box tests pass in the new package. The tests that need the real host stay in `main`.

## Compatibility
The exit codes, the usage text and the output of each verb are the same. The registry is unchanged.

## Limitations
The test fixtures in `main` repeat a few helpers of the new package. A shared fixture package is a later step.
