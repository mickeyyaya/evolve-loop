# Build Explanation — Cycle 2240

## Build Binding
- Cycle: 2240
- Base SHA: 9f0434cc0a4f87bf4609d8abfde5b80497b383d3

## Summary
This change tracks inbox item retry cycles as rounds and adapts retry strategies. The second run carries findings and raises effort. The third run forces a fresh deep plan.

## Rationale
Repeated replays with identical plans waste cycle budget. Forcing strategy shift on round three breaks failure loops.

## Changed Areas
- `agents/evolve-triage.md` — adds guidelines for triage to read retry plans and forbid earlier plans on fresh-deep-plan items.
- `go/internal/cycleoutcome/cycleoutcome.go` — forwards failure reasons and ceiling policy to inboxmover lifecycle outcome.
- `go/internal/inboxbatch/fieldowner.go` — registers retry_plan and last_failure_findings as loop-stamp owned fields.
- `go/internal/inboxmover/lifecycle/item.go` — records last failure fingerprint and stamps retry plans during count bumps.
- `go/internal/inboxmover/lifecycle/quarantine.go` — clears retry plans and findings upon quarantine release.
- `go/internal/inboxmover/lifecycle/release.go` — routes items to console on ceiling when configured and passes failure findings.
- `go/internal/inboxmover/lifecycle/round.go` — schedules item round numbers, findings carryover, and strategy change decisions.
- `go/internal/inboxmover/outcome.go` — propagates retry ceiling policy and failure reasons into release policy.

## Design Decisions
Round planning logic lives in a pure function PlanRound. Item stamps use existing loop-stamp mechanisms.

## Verification
ACS suite and unit tests verify plan changes, console routing, quarantine behavior, and full function coverage.

## Compatibility
Existing inbox items without retry stamps remain fully compatible and default to standard behavior.

## Limitations
Automatic multi-task inbox splitting remains deferred to a future convergence cycle.
