# ADR-0089 — Retirement releases the continuation binding; adoption refuses retired scopes

- **Status:** Accepted (2026-08-17, cycle-1507)
- **Driving incidents:** cycles 1484 / 1487 / 1497 (batch-20260816b/d) — a parked
  scope re-dispatched for three consecutive waves with no adoption event and no
  carryover entry.
- **Amends:** [ADR-0085](0085-continuation-registry-release.md), whose closing
  line names exactly this gap ("PASS-ship release … remains a queued
  optimization"). ADR-0076 slice C semantics (claims first, lane scope second)
  are unchanged.
- **Sibling:** PR #466 (transactional inbox consumption) — same disease, one
  store over.

## Problem

Dispatch reads **two** stores: inbox items and the scope-keyed
`continuation-registry.json`. ADR-0085 gave the registry a delete path but wired
it at ONE site — the orchestrator's adoption-decline branch. Every path that
takes an item OUT of the pending pool (`inboxmover.Promote` →
processed/rejected/retry/quarantine, `ReconcileSuperseded`, and ship-time
`consumeCommittedItems`) touched only the inbox store, so retirement was
one-sided and the binding stayed immortal.

The wave planner mints lane scopes from those bindings, so the registry is a
first-class lane SOURCE, not merely an adoption cache. Cycle-1487:
`context-fill-telemetry-and-cap` was parked out of `.evolve/inbox` (tracked
deletion shipped `d3c69cd2`), the next wave minted a lane straight off
cycle-1484's binding, burned a third lane on the same deterministic collision,
and re-registered itself (`9813bc62`) for a fourth.

## Decision

**1. Retirement releases, transactionally, pointer-first.** Every pool exit
releases the binding in the same operation that moved the item:

- `inboxmover.releaseContinuationOnRetire` (`continuation_retire.go`) runs at
  the end of a successful `Promote` — so all four destinations and
  `ReconcileSuperseded` (which goes through `Promote`) are covered by one call
  site, not five.
- `ship.consumeCommittedItems` does the same for ship-time consumption, against
  the ROOT-owned registry (`opts.ProjectRoot`) even when the consumption itself
  happens in a ship worktree.

Ordering is preserve-then-release: the binding VALUE is written onto the retired
item as `released_continuations[]` (worktree/branch/snapshot/base/findings +
`released_at`/`reason`) *before* the delete, so a crash between the two leaves a
live binding plus a preserved copy rather than an orphaned snapshot. The delete
is `DeleteRegistryEntryIfCycle`, never the unconditional one: a sibling lane
that rebound the scope between the read and the release keeps its fresh binding
(ADR-0085's TOCTOU rule).

**2. The read side refuses retired scopes.** `ResolveContinuationForScope` — the
ONE seam both the planner's lane-scope minting and post-triage adoption go
through (injected at `cmd/evolve/cmd_cycle.go` into
`core.WithContinuationResolver`) — now checks the scope before trusting a
registry hit, logs the refusal, and releases the dead binding so it cannot
re-arm. Its stderr is wired to `os.Stderr` at the injection site; it was the
`io.Discard` default, which is how three waves stayed invisible.

**3. "Retired" means positive evidence, not absence.** A binding is refused when
the scope id is found in a pool-exit subtree — `consumed/`, `quarantine/`,
`processed/`, `rejected/`, `retry/` (recursively: `processed/` and `rejected/`
nest a `cycle-N` level) — and NOT in the inbox root or a `processing/cycle-*/`
claim.

## Why not "absence = dead"

The obvious reading — live iff the batch loader can reach it, dead otherwise —
overreaches. The wave planner also mints lane scopes from **carryoverTodos**,
which never have an inbox file at all; that is precisely the cycle-1078 orphan
class the scope-keyed registry was added to serve (ADR-0076 slice C G2). Under
absence-as-death every carryover lane's preserved work would be released out
from under it: the re-dispatch defect traded for a salvage-loss defect. Two
production contract tests (`internal/core/continuation_lanescope_test.go`) pin
that shape and correctly rejected the stricter guard.

The residual cost is stated rather than papered over: an item whose file is
deleted outright leaves no evidence for the belt to find. That case is closed on
the **write** side by decision 1 — which is the primary fix; the guard is the
belt that holds when a release call is missed, not a replacement for it.

## Consequences

- Retirement is symmetric across both dispatch stores; a parked or consumed
  scope cannot re-dispatch from an immortal binding.
- The salvage pointer survives every release — `released_continuations[]` on the
  retired item is the operator's (and a future `evolve continuation` CLI's)
  trail back to the preserved snapshot.
- Anti-overreach is pinned in both directions: live root items, in-flight
  `processing/` claims, and lane-scope-only (carryover) bindings are all still
  adopted and never released.
- The third dispatch source — carryover todos — still lacks a retirement check
  (`carryover-lane-retirement-verifiableby`), and `evolve continuation
  list/release` (this item's third acceptance criterion) is deferred to the next
  cycle.
