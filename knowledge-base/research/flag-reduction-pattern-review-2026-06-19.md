# Flag-Reduction Campaign — Design-Pattern & Clean-Code Adherence Review

**Date:** 2026-06-19 · **Branch:** flag-reduction-v20 (cycles 15, 22–39) · **Scope:** registry 154→73 (−81 flags)
**Reviewer verdict:** ✅ **STRICTLY adheres** to the prescribed design patterns and clean-code guidance. No gaming, no capability loss, no dead-ref residue found in the sampled-and-traced changes. Adherence is *structurally enforced*, not incidental.

## Methodology
Reviewed the actual cycle diffs (`git show`), not summaries. For each pattern: (1) confirm the `os.Getenv`/`envchain` reader was *deleted*, (2) confirm the value was *re-homed* to the prescribed structure, (3) confirm a *consumer reads the new home* (capability preserved), (4) confirm *no orphaned reference* to the removed flag remains. Cross-checked against the campaign's own anti-gaming gate (flagreaders guard, FlagCeiling ratchet, grep-reader-gone).

## Pattern-by-pattern findings

### 1. Config-as-code (persistent dials/gates) — ✅ exemplary
Cycle 39 (`8b7137a5`) re-homed 7 flags (`CONSENSUS_AUDIT`, `REQUIRE_INTENT`, `TEST_PHASE_ENABLED`, `TRIAGE_DISABLE`, `BUILD_PLANNER`, `PLAN_REVIEW`, `SWARM_PLANNER`) into typed `policy.WorkflowConfig` fields. Clean-code quality observed:
- **Pointer fields for omit-vs-explicit** (`AutoPrune *bool`, `ConsensusAuditEnabled *bool`) — resolver guards on `!= nil`, so absence ≠ false. This is the correct three-state config idiom, matching the proven `FanoutConfig()`/`ObserverConfig()` shape.
- **Map consolidation (DRY):** the per-phase enables collapse into one `PhaseEnables map[string]string` rather than N parallel bool flags — eliminates duplication, the `never_duplicate_centralize` rule applied.
- **Consumed with documented precedence:** `cyclerun.go:237` — `caller > policy.PhaseEnables["intent"]=="on" > false`. Capability preserved, precedence explicit.
- **Reader genuinely gone:** all 4 traced flags → 0 live `os.Getenv` readers; consumer files (`cmd_consensus_dispatch.go`, `build.go`, `config.go`) wired to the policy value.

### 2. Split-const IPC/bootstrap reclassification — ✅ correct + auditable
`subagent/recursion.go:28` carries the mandated justification: `// SSOT IPC-protocol-allowed: parent→child recursion-depth handoff` (same pattern as `FanoutWorkerTokenEnv`). 9 split-const `"EVOLVE_"+...` sites; these are documented protocol/bootstrap handoffs reclassified *out* of the feature-flag registry, not hidden env reads. Matches the taxonomy's bucket-5/bucket-6 rule.

### 3. Clean removal — ✅ no residue
Removed flags (`BUILD_PLANNER`, `SWARM_PLANNER`, `PLAN_REVIEW`) leave **0 stale non-test references**. No commented-out husks, no orphaned doc rows (`control-flags.md` regenerated in lockstep, drift-checked).

### 4. Net code shape — ✅ consistent with re-homing
17 cycles: **+1100 / −736 = net +364 lines.** A gaming-by-deletion campaign would be net-negative with no structural additions; the positive delta is the config structs, resolvers, DI seams, and split-const protocol sites — i.e. capability *relocated*, not dropped.

## Anti-gaming verification
- Every traced removed flag: reader deleted, value re-homed, consumer reads new home, no stale ref. The `flagreaders` ACS guard (green each ship) mechanically forbids an orphaned reader, so a removed-row-with-live-reader cannot pass the gate.
- The `EVOLVE_AUDITOR_TIER_OVERRIDE` momentary dual-source suspicion was traced and **cleared**: 0 registry rows, 0 live readers, value in `policy.go` (3 refs) — fully migrated.
- `FlagCeiling` monotonic ratchet (`== len(All)` each cycle) prevents silently re-adding flags.

## Why adherence is consistent (not luck)
The discipline is enforced *structurally* every cycle by the campaign's own gates: `flagreaders` guard (no orphan readers), `FlagCeiling` ratchet (monotonic), grep-reader-gone audit predicate, and `control-flags.md` drift check. The pattern is the only way through the gate — the goal text mandates "pick exactly one bucket BEFORE editing," so the design pattern is chosen up front, not retrofitted.

## Minor notes (NOT violations)
- **In progress:** 73 rows remain. The un-migrated readers (`BRIDGE_*_DIR` bootstrap, `BUILD_PERMISSION_MODE` per-phase, `BYPASS_*` transient) are *sequenced for later clusters* (bootstrap is explicitly LAST), not skipped. Expected mid-campaign state.
- **~5 rows** have no `os.Getenv` reader = dead or already-reclassified protocol/bootstrap (not counted as operator flags) — consistent with the taxonomy.
- Recommendation for the tail: the hardest remaining clusters (bootstrap path-locators, rollout-stages `config.Load` inversion) are the load-bearing ones — keep the same per-flag "reader-gone + re-homed + consumer-wired" discipline; the gates will continue to enforce it.

## Bottom line
The flag-reduction changes **strictly follow** config-as-code / profiles-SSOT / DI / split-const / CLI-flag patterns and clean-code design (typed config, omit-vs-explicit pointers, DRY map consolidation, documented precedence, auditable IPC justification, zero dead-ref residue). The remaining work is correctness-of-sequencing, not pattern compliance.
