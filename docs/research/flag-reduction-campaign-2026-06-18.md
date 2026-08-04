# Flag-Reduction Campaign — Strategy & Goal Anchor (2026-06-18)

> **Request (user, verbatim):** "Could you also review all the flags to see if we can
> largely reduce the flag usage? The design of flag is easy to be broken and hard to
> maintain, do a deep dive on how to reduce the flag … use /evolve for deep-dive and
> planning for flag reduction and include in 4 consecutive loop testing."
>
> This file is the **campaign anchor** (landscape + reduction classes + principles +
> the loop goal-text). The per-cycle deep-dive and planning are owned by the
> evolve-loop's Scout/Triage/Plan-review phases — this doc scopes the goal; the loop
> does the work. The cycle-355 dual-root fix (ADR-0053) that hardened one flag
> break-mode landed first as the prerequisite.

## Landscape (registry snapshot, `go/internal/flagregistry/registry_table.go`, 300 flags)

| Status | Count | Meaning | Reduction signal |
|---|---:|---|---|
| `active` | 80 | real production flags, documented | keep; consolidate clusters where a config-object fits |
| `internal` | 111 | "undocumented production reader (2026-06-11 inventory); classify when touched" | **classify** → many are dead → remove; real ones → promote+document |
| `test-seam` | 65 | read only by `_test.go` | **relocate** out of the production registry (they are not product flags) |
| `dead` | ~38 | "no reader on any surface (2026-06-11 inventory)" / deprecated no-ops | **remove** (registry row + any vestigial reader + docs) |
| `deprecated` | 5 | WARN-bridges with `RemoveIn` targets, several past due (v8.61+, v18.x) | **retire** the bridge + delete the flag |

**Core problem the user named:** only **80 / 300 (27%)** are active production flags. The
other 73% — unclassified internals, test-only seams, dead no-ops, overdue deprecated
bridges — are the "easy to break, hard to maintain" surface. Each flag is a branch a
predicate/doc/gate can read from the wrong root (cycle-355) or drift against.

## Concrete reduction classes (lowest-risk first)

1. **Dead-flag removal (lowest risk).** ~38 `StatusDead` rows. Biggest single cluster:
   the **Budget Cluster** (~13 flags) — all `DEPRECATED no-op` from the token-budget
   removal (PR #96): `EVOLVE_BATCH_BUDGET_*`, `EVOLVE_BUDGET_*`, `EVOLVE_CHECKPOINT_*_PCT`,
   `EVOLVE_MAX_BUDGET_USD`, `EVOLVE_PHASE_COST_CEILING`, `EVOLVE_BUILDER_COST_*`,
   `EVOLVE_FANOUT_PER_WORKER_BUDGET_USD`. "Accepted but ignored" → safe to delete entirely.
   - **MANDATORY per-flag verification before removal (now MECHANICALLY ENFORCED):** the
     "no reader" claim is a 2026-06-11 hint, NOT proof. Grep every surface — `go/`,
     `.github/`, `skills/`, `agents/`, any `*.sh` — and paste the literal command + its
     output as removal evidence. The Go-only scan that classified cycle-360's
     `EVOLVE_INNER_SANDBOX` "dead" (while it had a live `adapters/claude.sh` reader) is the
     canonical false-dead FAIL. The `flagreaders` ACS guard
     (`go/acs/regression/flagreaders/`) now scans ALL these surfaces, so removing a row that
     any surface still references FAILs at the gate — but verify first; do not lean on the
     gate to catch your own removal.
2. **Deprecated-bridge retirement.** The 5 `StatusDeprecated` flags + their WARN-bridge
   readers, where `RemoveIn` has passed. Remove the bridge code AND the flag together.
3. **`internal` classification.** For each of the 111: grep readers across all surfaces.
   Zero readers → reclassify dead → remove. Real reader → promote to `active` with a
   one-line `Doc` + Cluster. This shrinks the "classify when touched" debt monotonically.
4. **Test-seam relocation (architectural).** The 65 `test-seam` flags are not product
   config — move them out of the shipped flag index (e.g. a build-tag-gated test registry)
   so `control-flags.md` documents only real flags.
5. **Cluster consolidation (architectural, highest value/effort).** Where a cluster of
   flags is really one decision, fold into a config-object / Strategy / Specification per
   the no-flag-sprawl rule (candidates: Budget once emptied; Workflow Defaults=19;
   Fan-out=13 — note Fan-out is marked "intentionally separate", respect that).

## Principles (constraints for every cycle)

- **No behavior change for `active` flags.** Removals target dead/no-op/test-only only.
  Every removal gets a regression guard (assert flag absent; assert behavior unchanged).
- **Verify readers across ALL surfaces before removing** — `go/`, `.github/`, `skills/`,
  `agents/`, `*.sh`. The 2026-06-11 inventory is a hint, not proof (root-cause/verify rule).
  Now enforced by the broadened `flagreaders` guard (cycle-360 lesson: a Go-only "dead"
  verdict for a flag with a non-Go reader is a false-dead). Two-part-removal applies to ANY
  surface: if a non-Go reference remains, delete the reference first, then the registry row.
- **Single-source-with-projection** (no_feature_flag_sprawl): consolidate via design
  patterns, never add a parallel flag. `control-flags.md` regenerates from the registry
  (now dual-root-correct after ADR-0053).
- **Integrity floor untouched.** Gate flags (`EVOLVE_EVAL_GATE`, `EVOLVE_CONTRACT_GATE`,
  EGPS, bypass hatches) are `active` and stay; consolidation must not weaken a gate.
- **Batch low-risk reductions; isolate risky ones.** Dead-flag removal and internal
  classification are pure registry/doc edits with no behavior change → BATCH many per cycle
  (they share the single registry file, so batching is the throughput lever; concurrency
  can't help — `fleet`'s `Partition` clusters same-file work into one cycle, partition.go:46).
  Behavior-changing or architectural reductions (deprecated-bridge retirement, test-seam
  relocation, cluster consolidation) stay ONE per cycle for isolated audit/rollback. Every
  removal/promotion ships with a regression guard; ship via the cycle pipeline.

## Loop goal-text (verbatim, for `evolve loop --goal-text`)

> Reduce the evolve-loop flag surface and improve its maintainability. The flag registry
> (`go/internal/flagregistry/registry_table.go`, projected to
> `docs/architecture/control-flags.md`) has 282 flags but only 82 are active production;
> 111 are unclassified `internal`, 65 are test-only `test-seam`, 18 are `dead`, 6 are
> `deprecated` (2 past-due). All reductions edit the single shared registry_table.go and the
> generated control-flags.md, so they cannot be parallelized across cycles — BATCH low-risk
> reductions into ONE cycle for throughput instead. Each cycle, do ONE of:
> (a) DEAD-FLAG SWEEP — in one cycle remove EVERY flag you can confirm dead this cycle:
> verify ZERO readers across ALL surfaces (go/, .github/, skills/, agents/, *.sh, go/Makefile,
> root instruction .md) and paste the grep per flag; the "no reader (2026-06-11 inventory)"
> note is a hint, re-verify. Give each removed flag its own regression guard. If a flag fails
> cross-surface verification, EXCLUDE it from the batch and proceed with the rest — never let
> one bad flag sink the cycle.
> (b) INTERNAL-CLASSIFICATION SWEEP — in one cycle classify a batch of ~15-25 `internal` flags:
> zero-reader → remove (with guard); real reader → promote `active` with a one-line Doc +
> Cluster. Batch freely; all low-risk.
> (c) DEPRECATED-BRIDGE RETIREMENT — ONE per cycle (removes a live WARN bridge + its reader
> code = behavior-adjacent; isolated audit/rollback).
> (d) TEST-SEAM RELOCATION or CLUSTER CONSOLIDATION — ONE architectural change per cycle,
> isolated (ultrathink + strong review).
> Prefer (a) then (b) for bulk throughput; choose (c)/(d) when (a)/(b) are exhausted or a
> past-due bridge / architectural win is higher-value. NEVER change behavior of active flags
> or weaken an integrity gate (EVOLVE_EVAL_GATE, EVOLVE_CONTRACT_GATE, EGPS, bypass hatches);
> every removal/promotion ships with a regression guard proving the flag is gone (or correctly
> reclassified) and behavior is unchanged. Regenerate control-flags.md from the registry.
> Justify with Clean Code + GoF patterns. Ship ONE batch (one coherent reconciliation,
> possibly many flags) per cycle.

## Status

- Prereq landed: **ADR-0053** dual-root fix (`4ad4c7c3`) — `flags`/`skills check` now
  validate the worktree, so a flag-reduction cycle that regenerates `control-flags.md`
  no longer hits the cycle-355 trap.
- Cycles 356–359 SHIPPED (300→280); cycle-360 FAILED audit (false-dead: removed
  `EVOLVE_INNER_SANDBOX`/`EVOLVE_FORCE_INNER_SANDBOX` that still had live `adapters/claude.sh`
  readers). Damage remediated (flags → `StatusDeprecated`; the script→Go migration then
  deleted all `.sh`). Registry now at **282** (82 active / 111 internal / 65 test-seam /
  18 dead / 6 deprecated).
- **Blocker root-caused + fixed (Phase 0):** the false-dead class was SYSTEMIC (2nd after
  cycle-352) because the `flagreaders` guard + Scout grep scanned Go ONLY. The guard now
  scans `.github/`, `skills/`, `agents/`, `*.sh` too (`go/acs/regression/flagreaders/`,
  reproduction test `TestScanTextTree_DetectsNonGoOnlyReference`). 0 orphans on main.
- Next: resume `evolve loop` at cycle 361 with the goal-text above, self-paced via `/loop`
  with parallel worktree-agent fan-out; ultrathink-fix any failure class before continuing.
