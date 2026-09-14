# Build Explanation — Cycle 1678

## Build Binding
- Cycle: 1678
- Base SHA: e2819460aad67d4fa083fb9df45803280e3de2c2

## Summary
The operator's own `evolve inbox batches` command now renders console-routed
(operator-owned) inbox work through the same `inboxbatch.PartitionConsole`
split that triage and the claim floor already use, so routed items leave the
selectable lane listing and arrive with the partitioner's own reason on both
the text and the `--json` path. Carried in the same base-bound diff is the
cycle-1677 ledger work this lane resumes: a successful `evolve ledger verify`
now states WHICH history it validated — full-strict from genesis, or strict
from a named epoch anchor whose preserved prefix an operator adjudicated.

## Rationale
Routing authority has been typed plumbing since ADR-0074: `PartitionConsole`
splits the backlog into lane-dispatchable and console-routed with one reason
per routed item, `internal/phases/triage/triage.go` consumes it, and
`inboxmover.Claim` refuses a lane draw of a routed item with exit 3. The one
consumer that did not was the command an operator actually types, which ran
`inboxbatch.Classify` over every loaded item — presenting operator-owned work
as a selectable batch with no reason and no separation. Reusing the existing
partition rather than adding a second classification rule is the smallest
change that closes the gap and keeps one routing authority: a second rule
could drift from the claim floor and re-open the cycles-1034/1035/1036 burn.
Wiring it into the text renderer alone was rejected because every machine
consumer of `--json` would still read operator-owned work as dispatchable.

For the ledger half, a verification that accepted every byte from genesis and
one that resumed at an adjudicated epoch anchor were two very different claims
reported with one string (`OK: chain intact`) — the shape that let the
ledger-1740 damage stay invisible. Returning the scope from the same single
chain walk, rather than resolving the anchor a second time for the message,
means the scope reported is necessarily the scope verified.

## Changed Areas
- `go/cmd/evolve/cmd_inbox.go` — partitions the loaded backlog with
  `inboxbatch.PartitionConsole(items, guards.IsProtectedSurface)` and classifies
  only the dispatchable half, so routed items can no longer appear as a
  `- batch …` line; adds a conditional trailing section listing each routed item
  with the partitioner's reason, and gives `--json` an object document
  (`batches` plus `console_routed`) so both output paths carry the same
  decision. The exclusion is loud because a silently narrowed backlog reads as
  full coverage.
- `go/cmd/evolve/cmd_ledger.go` — `evolve ledger verify` calls the scope-
  returning verification and prints the anchor it resumed from (entry_seq plus
  line SHA), or says it verified strictly from genesis, instead of one string
  for both claims.
- `go/internal/adapters/ledger/ledger.go` — adds `VerifyScope`, the existing
  `Verify` walk returning the scope it validated; `Verify` becomes a thin
  wrapper so no caller is forced to change.
- `go/internal/adapters/ledger/seal.go` — adds `VerifyDeepScope`, the deep
  counterpart, so the two production verification paths report the same
  provenance rather than disagreeing about what was checked.
- `go/internal/adapters/ledger/anchor.go` — adds the `VerifiedScope` type and
  makes `effectiveAnchorSHA` also return the resolved anchor line's own
  `entry_seq`, read from that line rather than from `ledger-anchor.json`, whose
  number is stale exactly when an in-band seal has moved the anchor past it.

## Design Decisions
The console bucket is rendered outside the `- batch …` listing rather than as
an extra batch: a batch is something a lane may draw, and an operator-owned
item is not. The `--json` document changes from a bare array of batches to an
object with `batches` and an omitted-when-empty `console_routed` array; no
in-repo consumer parses it, and a partition present in one renderer only is the
defect this change exists to close. Reasons are taken verbatim from
`PartitionConsole` (`route:console-…`, `protected fix surface: <path>`) rather
than re-derived at the render site, so the operator reads the same sentence the
claim floor enforces. The anchor identity is derived from the ledger's bytes,
never a literal, so two ledgers sealed at different lines report differently.

## Verification
`go test -tags acs -count=1 ./acs/cycle1678` — 11/11 PASS, including the two
that were RED at handoff (`TestC1678_001`, `TestC1678_011`) and the anti-no-op
control `TestC1678_002`, which fails any implementation that prints a console
section over a backlog with nothing routed. The native ACS suite reports
`green=180 red=0 skip=55 total=235`. `go test -count=1` is green on
`./cmd/evolve`, `./internal/inboxbatch`, `./internal/guards`,
`./internal/phases/triage`, `./internal/core`, `./internal/phases/ship` and
`./internal/adapters/ledger`; `go test -race -count=1 ./cmd/evolve` is green,
as are the predicate-driven race lanes over `./internal/inboxbatch` and
`./internal/adapters/ledger`. CI-parity apicover over both enrolled touched
packages reports 30/30 and 25/25 exported symbols covered, 0 false-green.

## Compatibility
`evolve inbox batches --json` now emits a JSON object (`{"batches": […],
"console_routed": […]}`) where it previously emitted a bare array of batches.
No in-repo consumer reads that output; the text header line and the
`- batch …` listing format are unchanged. `ledger.Verify` and
`ledger.VerifyDeep` keep their signatures and behaviour, so every existing
caller is unaffected; the scope-returning variants are additive.

## Limitations
The text worklist lists routed items as reason lines only — it does not group
them, rank them, or show their titles, so an operator triaging a large routed
backlog still opens the item files. `evolve inbox batches` remains a read-only
view: nothing here lets an operator re-route an item from the terminal.
