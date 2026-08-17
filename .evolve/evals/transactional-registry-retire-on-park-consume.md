---
score_cap:
  - criterion: "Parking an item releases its continuation-registry binding in the same operation, without touching unrelated live bindings"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_001_ParkReleasesRegistryBinding$' ./acs/cycle1507"
  - criterion: "The released binding VALUE is preserved into the retired item file as released_continuations[] — no salvage-pointer loss"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1507_002_ParkPreservesBindingPointerInItemFile$' ./acs/cycle1507"
  - criterion: "Ship-time consumption releases the consumed item's registry binding, proven by a named in-package regression test"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestConsumeCommittedItems_ReleasesRegistryBinding$' ./internal/phases/ship 2>&1 | grep -q '^--- PASS: TestConsumeCommittedItems_ReleasesRegistryBinding'"
---

# Eval: transactional registry retire on park / consume

> Pins the release half of the continuation-registry lifecycle. `DeleteRegistryEntry`
> has existed since the 2026-08-10 immortal-entries stall but nothing called it when an
> item left the pending pool, so the dispatch pool was TWO stores (inbox items and
> scope-keyed continuation bindings) and every retirement path touched only one. Live
> burn: batch-20260816b cycle-1487 — `context-fill-telemetry-and-cap` was parked out of
> `.evolve/inbox` (tracked deletion shipped `d3c69cd2`) and the next wave dispatched it
> anyway as an adopted continuation from cycle-1484's binding, burned a third lane on the
> same deterministic collision, and re-registered itself (`9813bc62`) for a fourth.
> This eval keeps the release wired at BOTH pool-exit paths and keeps the pointer
> surviving the release, so future cycles cannot regress either half silently.
> Source incidents: cycles 1484, 1487, 1497. Sibling: PR #466 (transactional inbox
> consumption — same disease, one store over).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| park-releases | Quarantine/park releases the binding; unrelated live bindings survive | 8/10 | `go test -tags acs -run TestC1507_001_...` |
| pointer-preserved | `released_continuations[]` on the retired item keeps snapshot/branch/base SHAs | 7/10 | `go test -tags acs -run TestC1507_002_...` |
| consume-releases | Ship-time consumption releases too (named in-package wiring proof) | 7/10 | `go test -run TestConsumeCommittedItems_ReleasesRegistryBinding ./internal/phases/ship` |
