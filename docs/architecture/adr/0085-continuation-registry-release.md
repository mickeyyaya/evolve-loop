# ADR-0085 — Continuation-registry bindings are released at the adoption-decline site

- **Status:** Accepted (2026-08-10, landed #428)
- **Driving incident:** [2026-08-10 continuation absorbing-FAIL](../../incidents/2026-08-10-continuation-absorbing-fail.md) (cycles 1412/1418)
- **Amends:** ADR-0076 (continuation-on-fail) — adds the missing lifecycle half; the ADR-0074/cycle-1285 gate semantics are unchanged.

## Problem

The root-owned `continuation-registry.json` (ADR-0076 slice C G2) records a
lane-scope → preserved-snapshot binding so the defect-ledger gate can witness
lineage out of band — an agent deleting its workspace manifest must not be able
to launder inherited defects (cycle-1285). But the registry had **no delete
path**: `WriteRegistryEntry` was the whole API. Meanwhile the adopter
(`adoptContinuationAfterTriage`) **declines** stale bindings — snapshot landed,
base no longer an ancestor, worktree gone — and provisions a fresh worktree
*without* writing a workspace manifest. Result: registry-binds + no-manifest,
which is exactly the shape the gate treats as tampering and blocks
unconditionally. Any scope whose work was ever preserved became a permanent
audit auto-FAIL (an absorbing state): each new FAIL re-stamped a binding, and
the chain never converged. 84 entries had accumulated; ~60 pointed at dead
state.

## Decision

The component that DECIDES a binding is stale is the component that releases
it, atomically with the decision, on the orchestrator side:

- `continuation.DeleteRegistryEntry(root, scope)` — unconditional release;
  flock-serialized read-modify-publish; absent scope is a clean no-op; empty
  scope rejected (mirrors the write path); shared `publishRegistryLocked` so
  write/delete publish semantics cannot diverge.
- `continuation.DeleteRegistryEntryIfCycle(root, scope, cycle)` — check and
  delete under ONE lock hold. The naive read-guard-then-delete across two lock
  acquisitions loses a sibling lane's concurrent rebind (adversarial-review
  BLOCK, pinned by test): a fleet wave's lanes write this one map
  concurrently, so the ancestor check must be inside the critical section.
- `releaseDeclinedBinding` (`internal/core/continuation_stamp.go`) runs in the
  adoption-decline branch only: releases each scope whose entry names the
  declined ancestor cycle, one loud `RELEASED stale binding` line per release,
  best-effort — a failed release degrades to the pre-fix behavior (decline
  again next cycle), never anything worse.

## Why not the alternatives

- *Write the manifest on decline too:* the manifest is workspace-owned and
  agent-writable — a forged "declined" manifest would become a laundering
  channel; the gate would have to trust the very artifact the cycle-1285
  shield exists to distrust.
- *Soften the gate's no-manifest block:* reopens cycle-1285 directly.
- *Registry TTLs / GC sweeps:* liveness is a decision the adopter already
  makes per-binding with full context; a time-based sweep would release
  bindings whose snapshots are still adoptable.

## Consequences

- Lane agents still have no write path to the registry; the gate's
  anti-tamper branch is byte-identical, its pins untouched.
- Dead entries self-drain loudly as lanes touch them — no manual registry
  surgery, no migration step.
- PASS-ship release (drop the binding when the scope's work lands) remains a
  queued optimization: after this ADR its absence costs one loud
  decline-release on the next dispatch instead of an absorbing FAIL.
