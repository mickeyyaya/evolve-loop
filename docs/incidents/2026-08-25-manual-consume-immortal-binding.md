# 2026-08-25 — Manual inbox consume leaves the continuation binding immortal (cycle-1558)

## Symptom

soak wave 3, cycle-1558: the fleet planner minted a lane scoped to
`premise-challenge-fail-never-reaches-failure-learning` — an item whose work
shipped in cycle-1552 (df322f6c) and whose FILE the operator had consumed the
previous day (`evolve inbox consume` → `inbox/consumed/`, fingerprint acked).
premise-challenge/audit correctly zero-delivery-FAILed the duplicate. A full
lane burned re-proving finished work for the SECOND time — cycle-1553's burn
was the inbox-file half of the class (#496); this is the binding half.

## Root cause

The immortal-binding class (cycles 1487/1497): the wave planner mints lanes
off scope-keyed continuation-registry bindings. The SHIP-path consumption
releases the binding in the landing (#496's transactional retire), but the
operator command `evolve inbox consume` only moved the file and acked the
fingerprint — the binding survived, and the next wave minted from it.

## Fix (mirrors the fingerprint-reconciler precedent exactly)

1. `evolve inbox consume` now releases the item's binding in the same
   invocation, through the ONE shared transaction
   (`inboxmover.ReleaseContinuationBinding`: preserve-then-delete, loud on a
   failed preserve, cycle-guarded delete) — review caught a hand-rolled first
   draft as exactly the cycle-1507-H2 drift the transaction exists to prevent.
2. `inboxmover.ReconcileConsumedBindings` — the consumed-corpus sweep,
   running beside `reconcileConsumedFingerprints` before EVERY cycle dispatch
   on the blocker-breaker path. Guarded by the cycle-1507 pair (measured 7/91
   live bindings a guardless release would have destroyed): a re-filed LIVE
   copy owns its binding, and a consumed copy OLDER than the binding is stale
   evidence — both skip, loudly. Self-heals the live stray that burned
   cycle-1558.
3. `continuation.AppendReleased` single-sources the released-pointer shape
   (host-path redaction included) for BOTH the ship path and these — the
   salvage pointer can never drift between routes.

## Regression pins

`cmd/evolve/cmd_inbox_consume_binding_test.go`: consume releases + preserves
prior released entries; the sweep releases strays and leaves live bindings;
the breaker-boot wiring pin (`blockerBreakerHalt` drives the sweep). Mutation-tested: 7 mutants killed (consume-path severed, sweep release
severed, shared-txn delete skipped, wrong-dir sweep, breaker wiring severed,
recency guard removed, live-copy guard removed).
