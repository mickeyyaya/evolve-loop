# Build Explanation — Cycle 2503

## Build Binding
- Cycle: 2503
- Base SHA: ebf677e65020027f6ed3b4fab1ecf1a39301a104

## Summary
The inbox verb group now lives in the package `internal/cli/inboxcmd`. The package `cmd/evolve` keeps one shim that passes a `Deps` value.

## Rationale
The move shrinks `cmd/evolve` and puts each inbox verb behind one typed entry point. A `Deps` value replaces the calls from the moved code back into `main`.

## Changed Areas
- `go/internal/cli/inboxcmd/deps.go` — defines the `Deps` value and the `host` accessor. The entry points `Run` and `RunMover` are in `cmd_inbox.go` and `cmd_inbox_mover.go` of the same package.
- `go/internal/cli/inboxcmd/doc.go` — holds the one package doc.
- `go/internal/cli/inboxcmd/cmd_inbox_add.go` — moved verb source, now in the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_curator.go` — moved curator source, now in the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_mover.go` — moved mover source, now in the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_quarantine.go` — the quarantine verb, now in the new package.
- `go/internal/inboxtest/inboxtest.go` — shared test fixture used by both packages.
- `go/cmd/evolve/cmd_inbox.go` — the shim that builds `Deps` and calls `inboxcmd`.
- `go/cmd/evolve/cmd_inbox_quarantine.go` — removed; the verb moved.
- `go/cmd/evolve/cmdutil/projectroot.go` — project-root helper shared by `main` and the new package.
- `go/cmd/evolve/cmd_cli_routing_write.go` — uses the shared helper after the move.
- `go/cmd/evolve/cmd_loop_blockerbreaker.go` — calls the moved code through the new package.
- `go/cmd/evolve/cmd_pr.go` — calls the moved code through the new package.
- `go/cmd/evolve/cmd_signals.go` — calls the moved code through the new package.
- `go/cmd/evolve/cmd_supply_split.go` — calls the moved code through the new package.
- `go/cmd/evolve/cmd_supply_wiring.go` — calls the moved code through the new package.
- `go/.apicover-enforce` — enrolls `./internal/cli/inboxcmd` and `./internal/inboxtest`.
- `go/.test-dup-baseline` — baseline updated for the moved tests.
- `docs/architecture/packages/internal-cli-inboxcmd.md` — package page for the new package.
- `docs/architecture/packages/internal-inboxtest.md` — package page for the fixture package.
- `go/evolve` — rebuilt binary.
- `go/cmd/evolve/cmd_inbox_ackfingerprint.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_add.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_blocked_by.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_claim.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_claims.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_consume.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_curate.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_curator_apply.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_curator_follow.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_curator.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_list.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_mover.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_rank_print.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_rank.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_route_console.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_route_lane.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_show.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_split.go` — removed from `main`; the verb source moved to the new package.
- `go/cmd/evolve/cmd_inbox_trailercheck.go` — removed from `main`; the verb source moved to the new package.
- `go/internal/cli/inboxcmd/cmd_inbox_ackfingerprint.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_blocked_by.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_claim.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_claims.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_consume.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_curate.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_curator_apply.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_curator_follow.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_list.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_rank_print.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_rank.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_route_console.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_route_lane.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_show.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_split.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox_trailercheck.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.
- `go/internal/cli/inboxcmd/cmd_inbox.go` — moved here from `cmd/evolve`; the verb now takes a `Deps` value.

## Design Decisions
A `Deps` value carries every dependency that the moved code used to take from `main`. This avoids an import cycle. `Run` and `RunMover` store the value in one package variable (`current`, an atomic pointer in `deps.go`), and the `host` accessor reads it. The package therefore keeps one global.

## Verification
The ACS suite for cycle 2503 reports verdict PASS with red=0. The tests of `cmd/evolve` and `internal/cli/inboxcmd` pass under `go test -count=1`.

## Compatibility
Each inbox verb keeps its flags, output and exit codes. The production binary behaves the same.

## Limitations
The curator tests that need the moved fixture now run in the new package, not in `cmd/evolve`.
