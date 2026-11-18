---
score_cap:
  - criterion: "An item whose declared files are unprotected but whose text names a protected file gets no lane reroute proposal, in the package and through evolve inbox curate"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2481_001 ./acs/cycle2481"
  - criterion: "An item whose named fix is already on main or whose premise is absent gets a consume or quarantine proposal and no reroute"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2481_002 ./acs/cycle2481"
  - criterion: "The 16 items of the wave 178 boundary give a reroute proposal for the 9 that lane triage kept and none for the 7 it bounced"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2481_003 ./acs/cycle2481"
  - criterion: "The production curator, with no injected hook, derives the freshness of the 3 stale wave 178 items from the tree and git and proposes the 9 kept items through evolve inbox curate"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestCmd_InboxCurate_(TheWave178BoundaryThroughTheProductionCurator|AStaleItemOfTheWave178Boundary|ANamedPremise|AnAlreadyShipped)' ./cmd/evolve"
---

# Eval: the curator checks the fix surface and the freshness before a reroute

> Pins the three acceptance criteria of the inbox item. At the wave 178 boundary the curator proposed 16 reroutes from the declared files alone, and lane triage bounced 7 of them in wave 179 (4 hidden protected fix surfaces, 3 stale items). Each predicate runs named tests that drive the real curator, and each keeps a control item that must still be rerouted, so a curator that proposes nothing cannot pass. Source incident: docs/incidents/2026-11-18-seven-rerouted-items-bounced-back-to-the-console.md; cycle 2481. Audit repair (round 2): the first build passed criterion 3 only through test-injected answer tables, and the production curator still rerouted 7 of 7 bounced items.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| hidden-surface | text naming a protected file blocks the reroute | 6/10 | `go test -tags acs -run TestC2481_001 ./acs/cycle2481` |
| stale | a fix already on main or an absent premise blocks the reroute | 6/10 | `go test -tags acs -run TestC2481_002 ./acs/cycle2481` |
| wave-178 | 9 kept proposed, 7 bounced not proposed | 7/10 | `go test -tags acs -run TestC2481_003 ./acs/cycle2481` |
| production-resolver | the same split through `evolve inbox curate` with no injected hook; a named premise that landed, or a commit that main lacks, does not stop a reroute | 4/10 | `go test -run TestCmd_InboxCurate_ ./cmd/evolve` |
