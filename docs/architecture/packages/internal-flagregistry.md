# internal/flagregistry

## Purpose

`internal/flagregistry` is the declarative SSOT for every `EVOLVE_*` control flag across every reader surface: Go production code, Go test seams, and the bash skill/agent/commit-gate surface. `cmd/evolve/cmd_flags.go` (`evolve flags generate`/`evolve flags check`) projects and verifies it against `docs/architecture/control-flags.md`, and the `go/acs/regression/{flagreaders,flagceiling,flagprogress,envtaint}` guards and the per-cycle ACS predicate suites (`go/acs/cycleN/predicates_test.go`) read it to hold flag classification and count to a ratchet.

## Design

- Metadata only — the registry never funnels env reads through `config.Load`: subprocess-reads-env is a deliberate architecture property (the bridge subprocess and bash adapters read their own env).
- The registry existed to close a measured 252-actual-vs-93-documented drift between the flags actually read across all surfaces and what `control-flags.md` documented; `evolve flags generate` projects the registry into that doc's marker region and `evolve flags check` fails on drift, so a flag can no longer ship undocumented.
- New flags get a new row in `registry_table.go`, kept sorted by `Name` (`Lookup` binary-searches it); the reader-completeness guard (`go/acs/regression/flagreaders`) catches a flag read in production Go that lacks a row.
- The registry intentionally carries no minimum-count floor: the flag-reduction campaign removes dead/test-only/duplicate flags over time, and a floor would block that.
- `FlagCeiling` bounds total registry rows (`len(All)`) but is not the campaign's progress metric — it is a completeness backstop that may rise only when the flagreaders guard finds a pre-existing unregistered live reader requiring a new row, never to mask a net-new feature flag.
- `LiveFeatureFlagCeiling` is the campaign's real monotonic-decrease ratchet: `LiveFeatureFlags()` (`StatusActive` minus core-infrastructure) must never exceed it, and the campaign target is 0 — the `no_feature_flags` goal, since cross-component behavior belongs in `policy.json`/DI/Strategy, never stays an env dial. The in-tree ceiling check is a fast backstop; `go/acs/regression/flagceiling` additionally fails the per-cycle gate when the live count rises against the main baseline, which a same-metric unit test alone cannot enforce.
- `LiveFeatureFlags` excludes Deprecated/Internal/TestSeam/Dead rows: deprecating a flag removes its live `os.Getenv` reader, so the row stops counting toward the metric even though its tombstone remains in `All` for back-compat documentation.

- **Status says how a flag is read.**

  | Status | Meaning |
  |---|---|
  | `active` | Read in production and operator-facing. Do not remove it without a deprecation window. |
  | `deprecated` | Still honored, often through a bridge to its replacement (`ReplacedBy`), but it emits a WARN. It is scheduled for removal (`RemoveIn`). |
  | `dead` | No reader on any surface. Its mentions can be deleted. |
  | `internal` | Read by production code but set by the runner itself (subprocess injection or plumbing). It is not an operator dial. |
  | `test-seam` | Read only by `_test.go` files. Never set in production. |

- **`Kind` is optional metadata** with the values `bool`, `int`, `string`, `enum`, `path` and `usd`. An empty `Kind` means unspecified. `Default` is optional too. `Cluster` mirrors the section of `control-flags.md` that documents the flag.
- **The retired shell-read flags and what replaced them.** `EVOLVE_INNER_SANDBOX` gave way to `EVOLVE_SANDBOX`. `EVOLVE_FORCE_INNER_SANDBOX` was a deprecation bridge, and both ends are gone. `EVOLVE_PROFILE_WORKTREE_AWARE` gave way to `BridgeRequest.Worktree`. `EVOLVE_REINVOKE_CMD` gave way to the `interactive_prompts` of the manifest.

## Invariants

- A flag name is `EVOLVE_` followed by upper-case words. A real name never ends in `_` and never holds `__`. Those shapes are grep artifacts: bash interpolations such as `EVOLVE_${PHASE}_MODEL` and doc wildcard prefixes such as `EVOLVE_BUILDER_*`. Pinned by the name pattern in `TestAll_NonEmptyAndWellFormed`.
- `All` stays sorted by `Name`, because `Lookup` binary-searches it and the generated doc must be stable. Pinned by `TestAll_SortedByName`.
- `FlagCeiling` and `LiveFeatureFlagCeiling` are one-way upper bounds. Pinned by `TestRegistry_FlagCeiling` and `TestRegistry_LiveFeatureFlagCeiling`.
- `len(LiveFeatureFlags())` equals the count of `StatusActive` rows minus the count of core-infrastructure rows. Pinned by `TestLiveFeatureFlags_EqualsActiveMinusCore`.
- The dead-flag sweep runs in the normal test run, not behind an `acs` build tag, so it is a permanent regression guard. Pinned by `TestDeadFlagsSweep_Gone`.
- `renderDoc` folds and escapes every branch of a row, which the table-wide test does not reach with real rows. Pinned by `TestRenderDoc_FoldsAndEscapes`.

- `ClusterCoreInfra` must stay identical to the `Cluster` string on the core-infrastructure rows, or `IsCoreInfra` silently classifies nothing as core (pinned by `TestIsCoreInfra_ClusterMarkerConstMatchesData`).
- Observer and inactivity tuning live in `policy.ObserverPolicy`; the registry never re-adds the retired `EVOLVE_INACTIVITY_*`/`EVOLVE_OBSERVER_*` names, which would recreate a second configuration surface (pinned by `TestAmplify_ObserverInactivityFlagsRetired`).
- Any Active flag whose name contains `OBSERVER` must carry a non-empty `Cluster`, so generated docs classify it (pinned by `TestAmplify_AllActiveObserverFlagsHaveCluster`).
- An Active flag must carry a non-empty `Cluster` and `Doc`, and never the internal "classify when touched" placeholder — this keeps the internal-classification effort honest (pinned by `TestAll_NonEmptyAndWellFormed`).
- `LiveFeatureFlags` preserves `All`'s by-Name sort order (pinned by `TestLiveFeatureFlags_SortedByName`).
- `RenderIndex` is deterministic — `All` is sorted and the renderer is pure — and covers every row (pinned by `TestRenderIndex_StableAndComplete`).
- A shell-read flag that left the registry silently does nothing for an operator who still sets it. The Go runtime ignores unrecognized env vars. The Go bridge replaced `adapters/claude.sh` entirely for every former use. Pinned by `TestFlagRegistry_MigratedShellReadFlagsAreAbsent`.

## Findings

- A grep-based "no reader anywhere" classification once misclassified four flags as dead while they were still read by the bash adapters (`adapters/claude.sh`); they stayed correctly `StatusDeprecated` until the script-to-Go migration deleted that reader, and only then were the rows removed. A registry classification must check every reader surface (Go and bash), not just Go's AST scan.
- `EVOLVE_WORKTREE_PATH` left the registry on 2026-10-26. No Go code reads or writes it. The 14 archived predicates that kept the row (`acs/cycle22` to `acs/cycle39`) said that `agents/evolve-tester.md` reads it as a live handoff. No process sets the name, so that read always falls back to the unprefixed `WORKTREE_PATH`. The `flagreaders` guard keeps the tester read as a retired name. `acs/cycle10` pins the removal.
