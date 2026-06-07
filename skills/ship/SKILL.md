---
name: ship
description: Use after audit returns Verdict PASS. Atomic git commit + tag + ledger update. Single-writer; cannot fan-out.
---

# ship

> Sprint 3 composable skill. Wraps the Ship phase. The atomic commit at the end of every successful cycle.

## When to invoke

- After `audit` returns Verdict PASS (or WARN with explicit override)
- Cycle is in `audit` phase, transitioning to `ship`

## When NOT to invoke

- Audit verdict is FAIL or ABORT
- The current tree-state SHA differs from what the auditor saw (cycle-binding violation)
- `EVOLVE_BYPASS_SHIP_VERIFY` is requested but not justified

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Verify audit verdict = PASS via `gate_audit_to_ship` | Gate passes |
| 2 | Run `legacy/scripts/utility/release.sh <version>` for consistency check | Markers consistent |
| 3 | Run `legacy/scripts/lifecycle/ship.sh "<commit message>"` | Atomic commit + tag created |
| 4 | Verify ledger entry added | `kind: "ship"` with cycle binding |

## Single-writer invariant

Ship is ATOMIC by design — even if other phases fan out, Ship cannot. There is one git commit per cycle. Concurrent ship attempts on the same cycle are blocked by `phase-gate-precondition.sh` (only one `active_agent: orchestrator` at ship phase).

## Cycle-binding (v8.13.0+)

`legacy/scripts/lifecycle/ship.sh` refuses to ship if the current tree-state SHA differs from the SHA captured at audit time (in the auditor's ledger entry). Prevents "audit cycle 50, ship cycle 51" exploits. This guarantee is preserved through Sprint 3's tri-layer refactor.

## Composition

Invoked by:
- `/evolve-loop:ship` (user-driven)
- `loop` macro after `/audit`

## Reference

- `legacy/scripts/lifecycle/ship.sh` (atomic commit + tag)
- `legacy/scripts/release-pipeline.sh` (full release lifecycle for `publish` operations)
- `docs/release-protocol.md` (vocabulary: push / tag / release / propagate / publish / ship)
- CLAUDE.md "Release & Publish Workflow"
