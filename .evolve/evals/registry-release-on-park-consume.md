---
score_cap:
  - criterion: "Parking an item releases its continuation-registry binding in the same operation, preserves the pointer into the parked item, and leaves unrelated bindings intact"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_001' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_001'"
  - criterion: "An item that never held a binding is parked with its JSON untouched — no invented released_continuations[]"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_002' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_002'"
  - criterion: "Ship-time consumption releases the consumed item's binding and preserves the pointer into the consumed item file"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestConsumeCommittedItems_ReleasesRegistryBinding$' ./internal/phases/ship | grep -q -- '--- PASS: TestConsumeCommittedItems_ReleasesRegistryBinding'"
---

# Eval: transactional registry retire on park / consume

> The pending-work pool has three dispatch sources — inbox items, carryoverTodos and
> scope-keyed continuation bindings — and for a long time only the first had a
> retirement path. `continuation.DeleteRegistryEntry` existed (added for the
> 2026-08-10 immortal-entries stall) but nothing called it when an item LEFT the pool,
> so a parked or consumed scope kept re-arming the wave planner from its immortal
> binding. Live burns: cycle-1487 (parked `context-fill-telemetry-and-cap` was
> dispatched anyway from cycle-1484's binding, burning a third lane on the same
> deterministic collision) and cycle-1497 (a consumed scope re-dispatched with no
> adoption event and no carryover entry). This eval pins the transactional retire on
> both pool exits, including the ordering rule that keeps the salvage pointer:
> preserve into the item file FIRST, release the binding second.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| park-releases | Park releases the binding, preserves the pointer, scoped to one item | 8/10 | `TestC1515_001` |
| no-overreach | Unbound items gain no annotation | 6/10 | `TestC1515_002` |
| consume-releases | Ship-time consume releases + preserves | 8/10 | `TestConsumeCommittedItems_ReleasesRegistryBinding` |
