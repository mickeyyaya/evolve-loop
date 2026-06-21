# Derived-Artifact Rebase Conflict Resolution (loop large-scale hardening)

**Date:** 2026-06-21
**Branch:** `feat/rebase-derived-artifact-resolution` (off main `67c14b98`)
**Status:** design-of-record for the fix
**Supersedes the merge-driver design** in the diagnostic workflow output (that design was
adversarially shown to be unworkable — see "Rejected approaches").

## Request

The serial flag-reduction campaign (proving run for "make the loop survive a difficult
large-scale project") ran 45 min / ~18 cycle attempts with **0 ships, 0 quarantined**,
`FlagCeiling` stuck at 39. Root-cause and fix the blocker, with strict TDD, clean code,
design patterns, and 100% coverage of touched logic. No feature flags; single-source-with-
projection; no workarounds (root-cause redesign).

## Root cause (verified — diagnostic workflow `w1x29i196`, 8 agents + 3 adversarial verifiers)

`docs/architecture/control-flags.md` is a **generated projection** of the
`internal/flagregistry` SSOT (`cmd_flags.go` — `evolve flags generate` splices
`flagregistry.RenderIndex()` into a marker region; `flags check` exits 2 on drift). The
generated region is ~80% of the file and is rewritten **wholesale** (`SpliceMarkedRegion`),
so two cycles that each remove a *different* flag produce textually divergent files that git
cannot 3-way-merge.

The campaign runs under `EVOLVE_FLEET=1` (needed for explicit per-cycle worktrees, even at
`--concurrency 1`). Worktrees are created `git worktree add … HEAD` at wave start; once an
earlier cycle in the wave ships, a later cycle's ff-merge diverges →
`CodeGitFleetRebaseNeeded` → the orchestrator rebases the cycle branch onto main
(`core/ship_recovery.go:rebaseCycleBranchOntoMain`). **That rebase conflicts on
`control-flags.md`**, and the current code treats *any* rebase conflict as "genuine
overlapping work the partition should have separated" → `rebase --abort` →
`CodeGitFleetRebaseConflict` (integrity) → debugger. The cycle never ships. Repeat for every
cycle ⇒ 0 ships, 0 quarantined (an abort is not a FAIL verdict).

The partition **cannot** keep flag cycles apart: every flag-reduction cycle touches the
registry + its projection. So the conflict is structural, not a partition failure — a
**derived-artifact merge hazard**.

## Critical correctness constraints discovered while grounding the fix

1. **Tree-SHA integrity binding** (`ship/gitops.go:492`, `:407`): the committed (and
   pre-commit staged) tree SHA must EXACTLY equal `internalAuditBoundTreeSHA`, set from the
   audit report (`ship/audit.go:124`). ⇒ The projection must NOT be regenerated *after*
   audit binds the tree (that would trip `CodeIntegrityTreeDrift`). It MUST be regenerated
   *before* re-audit. The fleet recovery routes `RebaseNeeded → audit` (re-bind), so
   regenerating during the rebase is integrity-safe: re-audit binds the regenerated tree.

2. **The renderer uses the COMPILED slice, not source** (`cmd_flags.go:54,66` —
   `flagregistry.RenderIndex()` / `len(flagregistry.All)`). The running campaign binary has
   the stale 39-flag set. Regenerating from the running binary would **re-inflate** the doc
   (undo the merged deletions). ⇒ Regeneration must reflect the **merged source**, which
   requires compiling it: `go run ./cmd/evolve flags generate` from the worktree's `go/`
   with `EVOLVE_WORKTREE_ROOT=<worktree>` (so `sourceRoot()` targets the worktree's doc —
   precedence `EVOLVE_WORKTREE_ROOT > EVOLVE_PROJECT_ROOT > cwd`, `cmd_subagent.go:478`).

## Chosen design — application-level resolve-by-regeneration

Pattern: **Strategy + Humble Object + single-source-with-projection.** The rebase-conflict
resolution IS the projection re-run (no second renderer; it shells the same
`evolve flags generate` command), so no duplicated logic.

In `core/ship_recovery.go`, when `git rebase main` conflicts:

- **All** unmerged paths are *declared derived artifacts* → for each, regenerate from the
  merged worktree source + `git add`, then `git rebase --continue` (looping across replayed
  commits; `git rebase --skip` for a now-empty commit). Return `ok=true` → re-audit re-binds
  the regenerated tree → re-ship fast-forwards. **Integrity-safe.**
- **Any** unmerged path is NOT a derived artifact (incl. the SSOT `registry_table.go`
  itself) → unchanged behavior: `rebase --abort` → `CodeGitFleetRebaseConflict` → debugger.
  A real SSOT/code conflict is genuine overlapping work and must not be auto-resolved.

### Testable decomposition (Humble Object)

```
gitFn  = func(ctx, dir, ...args) (stdout, exit, err)   // == gitCapture
regenFn = func(ctx, worktree, relPath) error            // regenerate one derived artifact

rebaseWithDerivedRegen(ctx, worktree, git gitFn, regen regenFn, isDerived func(string)bool) (ok, conflict bool)
```

Pure orchestration with injected git + regenerator → fully unit-testable with fakes, zero
real git/toolchain in the decision tests. `rebaseCycleBranchOntoMain` becomes thin wiring:
`rebaseWithDerivedRegen(ctx, wt, gitCapture, regenerateDerivedArtifact, isDerivedArtifact)`.

### Single classifier (generalizes; extensible)

```
var derivedArtifactRegenArgs = map[string][]string{
    "docs/architecture/control-flags.md": {"flags", "generate"},
}
```

`isDerivedArtifact(p) = _, ok := derivedArtifactRegenArgs[p]`. The production regenerator
looks up the per-path `evolve` subcommand and runs `go run ./cmd/evolve <args…>` in
`<worktree>/go` with `EVOLVE_WORKTREE_ROOT=<worktree>`. New projection ⇒ one map entry.
(Skills/agent docs — `skills generate` — are *not* registered here: the flag campaign never
touches them; registering them is a documented follow-up, not needed for this proving run.)
All new symbols are **unexported** (no apicover burden).

## Rejected approaches

- **Custom git merge driver (`.gitattributes merge=evolve-derived`)** — the diagnostic
  workflow's first proposal. Adversarially refuted: git merge drivers do **not** run under
  `git merge --ff-only` (ship's integration verb — it aborts on divergence without invoking
  any driver) **nor under rebase/cherry-pick**. The driver would essentially never fire on
  the real integration paths. Also needs git-config + install-path + `.gitattributes` +
  PATH-resolution machinery (large blast radius, silent-misconfig failure mode).
- **Regenerate post-merge / post-commit** — trips the tree-SHA integrity binding
  (`gitops.go:492`).
- **Regenerate from the running binary's `flagregistry.All`** — stale; re-inflates the doc.
- **Stop committing `control-flags.md`; generate at gate time** — breaks the docs-contract
  (doc must be committed + in sync) and changes the tracked-file semantics. Bigger change.
- **Provisional resolve (`--theirs`) + rely on re-audit to fix the doc** — re-audit routes
  to AUDIT (runs `flags check`), which would FAIL on the stale doc; audit does not regenerate.

## Invariants preserved

- docs-contract + `evolve flags check` stay green: the resolution writes EXACTLY what
  `flags generate` writes, so the committed tree is in sync with the registry at that commit.
- Ship integrity floor unchanged: the SSOT (`registry_table.go`) still merges via git's own
  text merge and is still audited; only the *derived projection* is auto-resolved, and only
  before re-audit re-binds the tree.
- single-source-with-projection: one renderer (the `flags generate` command); the resolver
  shells it, never reimplements `RenderIndex`.
- No feature flag: the classifier is a code map; behavior is unconditional in the recovery
  path.
- SSOT/code conflicts still route to the debugger (no silent auto-resolution of real logic).

## Secondary fix (orthogonal, same branch)

Campaign progress path nests inside a cycle worktree when `--project-root` is empty
(`cmd_campaign.go` `campaignEvolveDir` falls back to `os.Getwd()` un-absolutized; a detached
child chdir'd into its worktree resolves the relative path there). ⇒ `--resume`/`status`
never sees completed waves. Fix: absolutize the resolved dir (`filepath.Abs`) symmetric with
`*projectRoot`; refuse to write progress under any `.evolve/worktrees/` path (fail loud).

## TDD plan (RED first)

1. `rebaseWithDerivedRegen`: clean rebase → `(true,false)`.
2. all-derived conflict → regen+add+continue → `(true,false)`; assert regen + `git add` called for the path.
3. non-derived conflict (`registry_table.go`) → abort → `(false,true)`; regen NOT called.
4. mixed (derived + non-derived) conflict → abort → `(false,true)`.
5. multi-commit: `--continue` hits a 2nd derived conflict → regen again → `(true,false)`.
6. regen returns error → abort → `(false,false)`.
7. rebase fails with no unmerged paths (infra) → abort → `(false,false)`.
8. now-empty commit (exit≠0, no unmerged) → `--skip` → completes → `(true,false)`.
9. `isDerivedArtifact`: control-flags.md true; registry_table.go false.
10. map-integrity guard: every `derivedArtifactRegenArgs` key exists on disk + has a GENERATED marker pair.
11. (secondary) `campaignEvolveDir("")` is absolute and never under `.evolve/worktrees/`.
```
