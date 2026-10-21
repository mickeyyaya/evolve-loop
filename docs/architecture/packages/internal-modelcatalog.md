# internal/modelcatalog

## Purpose

`internal/modelcatalog` is the live, CLI-queried map from abstract model tier (`fast`, `balanced`, `deep`, `top`) to each installed CLI's concrete model id. It is the pure data layer. It holds the schema, the lookup, the staleness rule, the refresh assembly and the atomic store. It has no dependency on the bridge. The dispatch read path keeps the static manifest fallback at the call site, because importing the bridge here will create an import cycle.

## Design

- `Catalog` is keyed by base CLI name (`claude`, `codex`, `agy`). The caller strips driver suffixes such as `-tmux` and `-p`, so one entry serves every driver.
- `BuildFromSnapshots` is pure and deterministic. A CLI is included only when it is ready, named, and contributes at least one canonical-tier model. `CLISnapshot` keeps this package a leaf. The command layer adapts `setup.CLIStatus` into it.
- `Commit` is the one write seam for a refresh. It carries the operator-authored `TierFallbacks` forward from the catalog on disk, then writes. Callers must not compose `Read`, `MergeFallbacks` and `Write` themselves.
- `ReadShadow` and `WriteShadow` handle the shadow catalog. It is disposable output of the refresh pipeline in stage `shadow`. Dispatch never reads it.

- `DispatchModelChain(cli, tier)` gives the ordered concrete-model chain of a tier: `tier_models[tier]` first, then `tier_fallbacks[tier]`, with empty and repeated names dropped. It is empty for a missing or non-live entry, the same trust gate as `DispatchModel`. The bridge model failover reads it.

## Invariants

- Dispatch takes a model only from a `live`-sourced entry (`DispatchModel`). A `detect`-sourced entry is derived from the static manifest and is informational. Without this gate, a degenerate detect-derived catalog will override the proven manifest.
- A zero `FetchedAt` means never fetched, and it is always stale. A `FetchedAt` in the future (clock skew) counts as fresh, so the catalog does not refresh on every cycle.
- `Read`: a missing file is a zero catalog, which triggers a refresh. Malformed JSON is an error. A corrupt cache must surface, not look empty.
- `Commit`: a corrupt prior catalog is warned about and treated as empty. A refresh is the repair path for a corrupt cache. Refusing to write will strand the operator.
- `Commit` returns the catalog as persisted, with the carried chains merged in. Printing the input instead will show a catalog that lacks the chains just preserved.
- `MergeFallbacks` fills gaps only. A fresh entry that carries its own chain keeps it. A CLI absent from the new catalog is never restored to keep its old chain. The inputs are not mutated.
- `CanonicalTiers` drops non-canonical keys from a snapshot. The catalog never holds a tier that dispatch cannot request. `high` is an input alias of `deep` in the bridge, not a tier of its own.
- `CandidatesHash` fingerprints the decision inputs of the refresh. The next refresh skips the classifier call when the fingerprint matches. An absent hash never matches, so a pre-existing catalog classifies once and then stabilizes.

## Why the store writes the way it does

- `Write` keeps the outgoing catalog as `model-catalog.prev.json`, so one bad refresh can be rolled back. The live catalog is gitignored and partly hand-authored. An atomic rename with nothing retained will make a bad refresh unrecoverable.
- The retention is best-effort. A failure never blocks the write that matters.
- The retained copy is made with a temp file and a rename, for three reasons. Concurrent fleet lanes share one `.evolve`, and a torn `.prev` will turn the rollback into a second outage. `WriteFile` follows a pre-existing symlink at the retention path, but rename replaces it. This is the only write in the package that a planted link can redirect. `os.CreateTemp` yields mode 0600, and rename keeps that mode, so the copy keeps the owner-only access of the live catalog.
- `Write` rotates `.prev` only after the replacement is written and about to land. A failed marshal or write must not rotate it.
- The retention path calls `os.CreateTemp` directly, not through the `createTemp` seam. That seam exists so the failure-injection tests can drive `Write`'s error paths. Retention must not consume it.
- The shadow file rotates no `.prev`. Retention protects the hand-authored live catalog, and the shadow file is disposable.

## Findings

- The live `/model`-queried source will be a future producer of the same `Catalog` shape. `BuildFromSnapshots` does not change when that swap happens.
- `Efforts` empty means not discovered, not "no dial". The two are deliberately indistinguishable in the catalog. Recording a guess will make a missing rung look like an absent feature.
