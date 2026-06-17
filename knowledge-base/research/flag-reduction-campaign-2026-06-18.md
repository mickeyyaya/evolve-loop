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
   - **MANDATORY per-flag verification before removal:** the "no reader" claim is from a
     2026-06-11 inventory and may be stale or Go-only. Grep ALL surfaces
     (`go/`, `scripts/`, `skills/`, `*.sh`, `.evolve/`, configs) for each name. Note:
     bash sourcing-guards like `EVOLVE_RESOLVE_ROOTS_LOADED` / `EVOLVE_FAILURE_CLASSIFICATIONS_LOADED`
     are SET+READ at runtime if their script still exists — do NOT remove blind.
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
- **Verify readers across ALL surfaces before removing** — Go, bash, skills, configs.
  The 2026-06-11 inventory is a hint, not proof (root-cause/verify rule).
- **Single-source-with-projection** (no_feature_flag_sprawl): consolidate via design
  patterns, never add a parallel flag. `control-flags.md` regenerates from the registry
  (now dual-root-correct after ADR-0053).
- **Integrity floor untouched.** Gate flags (`EVOLVE_EVAL_GATE`, `EVOLVE_CONTRACT_GATE`,
  EGPS, bypass hatches) are `active` and stay; consolidation must not weaken a gate.
- **One well-scoped, test-backed reduction per cycle** with a regression guard; ship via
  the cycle pipeline.

## Loop goal-text (verbatim, for `evolve loop --goal-text`)

> Reduce the evolve-loop flag surface and improve its maintainability. The flag registry
> (`go/internal/flagregistry/registry_table.go`, projected to
> `docs/architecture/control-flags.md`) has 300 flags but only 80 are active production;
> 111 are unclassified `internal`, 65 are test-only `test-seam`, ~38 are `dead`, 5 are
> overdue `deprecated`. Each cycle, pick ONE highest-value, lowest-risk reduction:
> (a) remove confirmed-dead flags (verify ZERO readers across go/, scripts/, skills/,
> *.sh, configs — the "no reader (2026-06-11 inventory)" note is a hint, re-verify) plus
> any vestigial reader and the doc index; (b) retire a past-due deprecated WARN-bridge
> with its reader; (c) classify `internal` flags (dead→remove, real→promote+document);
> (d) consolidate a cluster of single-decision flags into a config-object/Strategy per
> the no-flag-sprawl rule. NEVER change behavior of active flags or weaken an integrity
> gate; every removal/consolidation ships with a regression guard proving the flag is
> gone and behavior is unchanged. Regenerate control-flags.md from the registry. Justify
> with Clean Code + GoF patterns. Ship one reconciliation per cycle.

## Status

- Prereq landed: **ADR-0053** dual-root fix (`4ad4c7c3`) — `flags`/`skills check` now
  validate the worktree, so a flag-reduction cycle that regenerates `control-flags.md`
  no longer hits the cycle-355 trap.
- Next: launch `evolve loop` with the goal-text above for 4 consecutive cycles; monitor
  each; ultrathink-fix any failure class before continuing.
