---
score_cap:
  - criterion: "The host claimer (inboxmover.ClaimPending, as cmd_cycle.go hostInboxClaimer calls it) refuses a pending item whose PendingDispatchability is not dispatchable (dependency pending, processing or retry), leaves it at the inbox root, names the item and its blocking dependency, and still claims a ready sibling"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_001_' ./acs/cycle1781/"
  - criterion: "`evolve inbox-mover claim <waiting-id> <cycle>` exits non-zero, names the blocking dependency, and leaves the item pending, while a dispatchable item still claims with exit 0"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_003_' ./acs/cycle1781/"
  - criterion: "The host claimer still claims items with no deps or with deps that are processed, consumed, rejected, quarantined or never filed, and still leaves a route:console-manual item pending without an error"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_002_' ./acs/cycle1781/"
  - criterion: "Every export of the touched apicover-enrolled packages (inboxmover, inboxmover/lifecycle, continuation) is named by a _test.go in its package"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_008_' ./acs/cycle1781/"
---

# Eval: the claim floor refuses an item whose declared dependency has not landed

> ADR-0106 W3 keeps dependency-blocked items off every lane menu
> (`PlaceOnLaneMenu` puts them in `MenuWaiting`). Nothing enforced the rule
> where a pick becomes a claim, though. The triage host claimer
> (`inboxmover.ClaimPending`, wired as `hostInboxClaimer` in
> `go/cmd/evolve/cmd_cycle.go`) and `evolve inbox-mover claim` both reached
> `inboxmover.Claim` → `lifecycle.Mover.Claim`, which checks console routing
> only. A triage that named a waiting id anyway (from a scout report, a
> carryover, or the dependency_blocked note) therefore claimed it and built
> it before its dependency landed. Design doc §7.7 W7; cycle 1781's bug
> reproduction confirmed it.
>
> The floor uses the same rule the menu uses (`PendingDispatchability`), so
> the menu and the claim cannot disagree. It names the blocker, as the
> console-routed refusal names its reason. A refused claim leaves the item
> at the root. The deliverable effects gate (`checkInboxClaim`) then reports
> the committed-but-unclaimed item, so the build cannot proceed ahead of the
> dependency.
>
> `inboxmover.go` and `inboxmover/lifecycle/` are protected surfaces
> (`guards/integrity_surface.go:95-96`). The predicates therefore drive the
> production claim callers, not the protected `Claim` facade.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| host caller | ClaimPending refuses the waiting item and names the blocker | 9/10 | `TestC1781_001_` |
| CLI caller | `evolve inbox-mover claim` refuses and names the blocker | 8/10 | `TestC1781_003_` |
| no over-block | dispatchable items claim; console items stay operator-owned | 7/10 | `TestC1781_002_` |
| apicover | touched enrolled packages name every export | 5/10 | `TestC1781_008_` |
