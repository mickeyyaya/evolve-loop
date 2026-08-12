---
score_cap:
  - criterion: "inboxmover.ClosesInboxIDs parses line-anchored Closes-Inbox markers (ids, dedup, order-stable)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestClosesInboxIDs ./internal/inboxmover"
  - criterion: "A marked inbox item is consumed by its own landing ship, including on a decision-less lane cycle"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestPromoteInbox_ClosesInboxMarkerConsumes ./internal/phases/ship"
  - criterion: "An unlanded ship, and a landing with no marker, consume nothing (cycle-598 gate reused; no closure inference)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestPromoteInbox_(ClosesInboxMarkerSkippedOnUnlandedShip|LandedShipWithoutMarkerConsumesOnlyTriageNamedItems|AbsentBuildReportIsNotAnError)' ./internal/phases/ship"
  - criterion: "The Closes-Inbox convention is documented for the Builder with the must-NOT-on-a-partial-landing caveat"
    max_if_missing: 5
    evidence: "grep -q 'Closes-Inbox:' agents/evolve-builder-reference.md && grep -q 'Closes-Inbox:' docs/architecture/inbox-injection-protocol.md"
---

# Eval: Consumption rides the landing ship (Closes-Inbox marker)

> Pins the transactional-consumption contract introduced in cycle 1452 for inbox
> item `consumption-rides-landing-ship` (weight 0.92, pipeline-repair). Consuming
> an inbox item used to be a separate act from the ship that closed it, so
> forgetting was always possible: overnight ~3-4 of ~20 cycles were
> bookkeeping-shaped, each burning a full scout→triage→tdd→build→audit→ship
> (~25-30 min) to move one JSON file. The class was already named by the
> 2026-07-20 forensics ("item-consumption must be transactional with landing",
> 949/948) and escalated 0.88→0.92 on a LIVE instance on 2026-08-12:
> `schema-aligned-salvage-layer` landed in #453 without its item being consumed,
> so wave cycle-1448 re-picked already-shipped work as live scope.
>
> The mechanism is a builder-authored, line-anchored `Closes-Inbox: <id>` marker
> in `build-report.md`, unioned into `promoteInbox`'s committed set under the
> EXACT pre-existing cycle-598 landing gate (`isLanded`). Two failure directions
> are capped, not one: under-consumption (the bookkeeping cycle) and
> over-consumption (silent data loss). `connects_to` is a hint, not an acceptance
> predicate, so closure is never inferred from the diff.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| marker-parse | Line-anchored parse, comma lists, dedup, prose rejected | 6/10 | `go test -run TestClosesInboxIDs ./internal/inboxmover` |
| consumption-rides-landing | Marked item consumed by its own landing, incl. decision-less lane | 8/10 | `go test -run TestPromoteInbox_ClosesInboxMarkerConsumes ./internal/phases/ship` |
| no-second-weaker-gate | Unlanded ship and unmarked landing consume nothing; absent report is not an error | 9/10 | `go test -run 'TestPromoteInbox_(…Unlanded…\|…WithoutMarker…\|…AbsentBuildReport…)' ./internal/phases/ship` |
| convention-documented | Marker + partial-landing caveat in persona and protocol doc | 5/10 | `grep -q 'Closes-Inbox:' agents/evolve-builder-reference.md …` |
