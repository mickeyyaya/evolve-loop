# internal/phaseinventory

## Purpose

`internal/phaseinventory` builds `.evolve/phase-inventory.json`. The file indexes every phase the pipeline can run. It holds the built-in registry entries and the user and plugin phases from all discovery roots.

The package is the phase counterpart of `skillinventory`. It shares that cache semantics: mtime freshness against a TTL, with a `--force` bypass.

## Design

- The inventory is advisory. The orchestrator re-reads the live merged catalog for routing and execution.
- So a stale inventory can weaken an advisor hint. It cannot break a cycle.
- `Options.ProjectRoot` is required. `RegistryPath` defaults to `docs/architecture/phase-registry.json` under it, and `Roots` defaults to `<ProjectRoot>/.evolve/phases`. Callers pass the expansion of `EVOLVE_PHASE_ROOTS` as `Roots`. `Result` carries the warnings of the fail-open scan: a missing registry, a malformed `phase.json` and a shadowed duplicate.
- Each entry has a `Source` (`builtin` or `user`), and a user phase has a `Root`, which is project-relative when the root sits inside the project. `CategoryIndex` maps a goal type to its phase names.
- The inventory is the public contract for the advisor and for any LLM that reads it offline. The LLM learns which phases exist before it mints one. The consumers are the advisor's catalog cards, `evolve phases list` and that offline reader.
- Every scan is fail-open. A missing registry or a broken overlay becomes a warning, never an error, because an index must describe what is loadable.
- Entries are sorted by name, not by pipeline order. The catalog keeps pipeline order; the index is a lookup surface, so name order reads better and produces deterministic diffs.
- A phase's derived contract (artifact, sections, verdict) is projected only when `phasecontract.SynthesizesContract` says the runtime resolver synthesizes one. A native executor with no declared outputs has none, so its entry shows none.

## Invariants

- A Control phase is still inventoried: the inventory is a catalog, not an advisor projection. Pinned by the `PhaseCount` of 3 in `TestBuild_AggregatesBuiltinAndUserRoots`, whose fixture holds one Control phase.
- A built-in phase without categories lands in `uncategorized`: it degrades and does not crash. Pinned by `TestBuild_CategoryIndex`.
- A missing registry is a warning, not an error. Pinned by `TestBuild_MissingRegistryFailOpen`.
- The cache honors the TTL and `--force`. Pinned by `TestBuild_CacheHitAndForce`. The name order of entries and categories is pinned by `TestSortByName_OrdersPhasesAndCategoriesByName`.
- The default TTL is one hour, matching the skill inventory cadence.
- `OutputFile` is the project-relative location; `Build` writes to `<ProjectRoot>/<OutputFile>`.

## Findings

- None open.
