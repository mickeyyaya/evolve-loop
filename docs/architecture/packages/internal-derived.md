# internal/derived

> Decision record: [ADR-0128](../adr/0128-landing-queue-tiered-reverification.md); spec: [fleet-landing-queue.md](../fleet-landing-queue.md) §7.1; plan: [concurrent-cycle-landing-2026-10.md](../../plans/concurrent-cycle-landing-2026-10.md) component Q1.

## Purpose

`derived` is the one catalog of the generated outputs. It tells which paths a generator writes, with which marker or region, from which inputs, and which `evolve` verbs generate and check them. Each consumer reads this catalog and keeps no second list. Today the consumers are the post-build normalizer (`core.normalizeDerivedProjections`) and the fleet rebase (`core.rebaseWithDerivedRegen`). The overlap proof (Q3) and the composition (Q4) read it next.

## Design

- **Three entries.** `flag-index` writes the `GENERATED:flag-index` region of `docs/architecture/control-flags.md`. `signal-codes` writes the `GENERATED:signal-codes` region of `docs/architecture/signal-codes.md`. `skill-projections` writes `commands/*.md`, `.codex-plugin/plugin.json` and `.agents/plugins/marketplace.json` as whole files, and the `GENERATED:phase-facts` region of `skills/*/SKILL.md`.
- **No entry for the router recipes.** `agents/evolve-router.md` holds the `GENERATED:goal-recipes` region, but no `evolve` verb writes it. A conflict there is genuine.
- **Inputs.** An entry has path inputs (a prefix that ends in `/`, or an exact path) and, at most, one set of Go inputs. The caller resolves the Go inputs with `ResolveGoInputs`. `CodeRegistrars` are the directories of the Go files that call `RegisterCode(`. `SkillcheckClosure` is the main-module closure of `go/internal/skillcheck` from `go list -deps`. When `go list` fails, `ResolveGoInputs` returns the error and keeps the registrars that it found.
- **Staleness.** `Stale(changed, dirs)` gives the entries whose inputs one change touched. The normalizer uses it.
- **Firing.** `Fires(lane, peer, conflicted, dirs)` gives the entries that fire for a composition (spec §7.1). An entry fires when a conflicted path is one of its outputs. It also fires when one side touched an input and the other side touched an input or an output. No consumer calls `Fires` yet. The composition and the overlap proof of the landing queue (Q3, Q4, Q13, Q14) read it.
- **The conflict class.** For a whole-file output with a marker, `IsDerivedConflict(path, merged)` is true only when both sides of the file carry the marker. Our side is the text outside the blocks plus the first part of each block. Their side is the text outside the blocks plus the last part. A JSON manifest has no marker, so its conflict is always derived.
- **The region class.** For a region output, `IsDerivedConflict` is true only when the file has the region and each conflict block lies strictly inside it. A file with no closed block is genuine.
- **Regeneration.** `Worktree` binds a generator, a git runner and a base. `Regenerate` runs the generate verb, then the check verb, then `git diff --check <base> -- <outputs>`. Exit 0 is clean. Exit 2 with a leftover conflict marker is an error, and exit 2 with only a whitespace report is not. Each other exit is an error, so a bad pathspec cannot pass the check.
- **Refresh.** `Refresh` runs the check verb first, and regenerates only when the check fails.
- **The generator is the one of the worktree.** The package does not run `evolve`. `core` passes a generator built on `WorktreeEvolveInvocation`, which runs `go run ./cmd/evolve` in the worktree with the worktree root key. The host binary never regenerates.
- **Markers.** A region output has a BEGIN and an END line. A Markdown whole file has `Output.Marker`. The JSON manifests carry no marker, because JSON has no comment.
- **Protected surface.** The package is on the protected surface (`guards/integrity_surface.go`), because it decides which conflicts the protected recovery regenerates. Pinned by `TestProtectedSurfaceManifest_CoversTheDerivedOutputCatalog`.

## Invariants

- **The catalog matches the tree.** Each output matches a file, and each whole-file Markdown output and each fixed region output carries its markers. Pinned by `TestCatalog_EveryOutputCarriesItsGeneratedMarker` and `TestCatalog_EveryInputExists`.
- **The skill projections are outputs.** Pinned by `TestCatalog_SkillProjectionsAreDerivedOutputs`.
- **The router recipes are no entry.** Pinned by `TestCatalog_TheRouterRecipeRegionIsNotAnEntry`.
- **A block inside the region is derived, and a block outside it is genuine.** Pinned by `TestRegionConflict_ABlockInsideTheRegionIsDerived` and `TestRegionConflict_ABlockOutsideTheRegionIsGenuine`.
- **Cross staleness needs both sides.** Pinned by `TestFires_CrossStalenessNeedsBothSides`.
- **A leftover conflict marker fails the regeneration.** Pinned by `TestRegen_RefusesALeftoverConflictMarker`.
- **The rebase regenerates a conflict in a command stub.** Pinned in `core` by `TestRebaseWithDerivedRegen_ACommandStubConflictRegenerates` and `TestRegen_UsesTheGeneratorOfTheWorktree`.

## Findings

- **The cycle-1841 class** (2026-10-09): a peer regenerated the 32 command stubs while a lane was open. The old rebase catalog held only `control-flags.md`, so a rebase whose only conflicts were in those stubs went to the debugger. Now the rebase regenerates the stubs with the generator of the worktree.
