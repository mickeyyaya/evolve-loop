---
score_cap:
  - criterion: "A lane naming another lane's held item in its decision and build report ships its own item and leaves the other in place"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestPromoteInbox_LaneNamingAnotherLanesItemShipsOwnAndLeavesTheOtherInPlace ./internal/phases/ship"
  - criterion: "Each refused id is named by a [ship] WARN line"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestPromoteInbox_RefusalWarnNamesEachRefusedID ./internal/phases/ship"
  - criterion: "A cycle without a lane pin cannot retire an item that another cycle holds"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestPromoteInbox_CycleWithoutLanePinCannotRetireAnotherCyclesHeldItem ./internal/phases/ship"
  - criterion: "A lane pin refuses an unheld inbox item outside the pin"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestPromoteInbox_LanePinRefusesAnUnheldInboxItemOutsideThePin ./internal/phases/ship"
---

# Eval: A shipped lane must not retire inbox items outside its scope

> Pins the host-fact scope guard in `promoteInbox`. The agent-written lists
> (`top_n`, `skip_shipped`, `superseded`, `Closes-Inbox:` lines) reach
> `Locate`, which scans every `processing/cycle-*/`, so a shipped lane could
> retire an item that a concurrent lane holds. Source: inbox item
> `promote-inbox-reads-agent-lists-outside-the-lane-pin` (finding F2 of PR
> #1290 round 2), cycle 2386.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | foreign-held item stays in place, own item ships | 5/10 | `go test -run TestPromoteInbox_LaneNamingAnotherLanesItemShipsOwnAndLeavesTheOtherInPlace` |
| warn | each refused id appears in a WARN line | 6/10 | `go test -run TestPromoteInbox_RefusalWarnNamesEachRefusedID` |
| no-pin | cross-cycle steal blocked without a lane pin | 6/10 | `go test -run TestPromoteInbox_CycleWithoutLanePinCannotRetireAnotherCyclesHeldItem` |
| pin | unheld root item outside the pin is refused | 7/10 | `go test -run TestPromoteInbox_LanePinRefusesAnUnheldInboxItemOutsideThePin` |
