# Build Explanation — Cycle 2546

## Build Binding
- Cycle: 2546
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
Premise drift now resets at `premise_verified_at`, and `ApplyNoWork` verifies stale, absent, and invalid drop evidence on the host and escalates repeated stale drops.

## Rationale
Using the existing `Item.VerifiedAt` reader and verifying drop evidence directly in `ApplyNoWork` closes the console-to-lane bounce loop with no extra phases or schema churn.

## Changed Areas
- `go/internal/phases/triage/premise_drift.go` — starts the `itemDrift` window at the later of `FiledAt` and `VerifiedAt` and labels verified items with the verification date.
- `go/internal/inboxbatch/fieldowner.go` — registers `evidence_verified` as a loop stamp and `stale_drop_count` as a loop counter.
- `go/internal/cycleoutcome/nowork.go` — invokes `stampDropVerification` during planned-no-work closeout before `RouteConsole`.
- `go/internal/cycleoutcome/drop_evidence.go` — verifies `stale` SHAs or files through git, verifies `absent` or `invalid` citations against `scout-report.md`, tracks `stale_drop_count`, and emits `INBOX_STALE_DROP_REPEATED` on a second stale drop.

## Design Decisions
`codeStaleDropRepeated` registers `INBOX_STALE_DROP_REPEATED` under `ModuleInbox` from `cycleoutcome` because `go/internal/inboxmover/lifecycle/lifecycle.go` is a protected surface. `evidence_verified` and `stale_drop_count` sit in `fieldowner.go` so `RouteLane` preserves the counter across console re-queues.

## Verification
All six cycle-2546 ACS predicates and the unit suites for `inboxbatch`, `phases/triage`, `cycleoutcome`, `inboxmover`, `signalcenter`, `repocontract`, and `testaudit` pass.

## Compatibility
Items without `premise_verified_at` keep their filing timestamp window, and non-stale or non-absent/invalid drops receive no `evidence_verified` field.

## Limitations
The optional planner pre-filter (part 3 of `premise-verification-anchor`) remains deferred per triage scope.
