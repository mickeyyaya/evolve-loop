# ADR-0061: Live-feature-flag campaign metric + monotonic-decrease gate

Status: Accepted
Date: 2026-06-22
Relates to: the flag-reduction campaign (no_feature_flags goal). Reframes `FlagCeiling` (introduced flag-reduction-v20) from *the* campaign metric to a loose completeness backstop.
Campaign: flag-campaign-7.

## Context

The flag-reduction campaign's progress metric was `FlagCeiling = len(flagregistry.All)` — the
total number of registry rows, ratcheted down each cycle. flag-campaign-7's cycle-5 exposed three
structural defects in that metric:

1. **Deprecation does not lower it.** The real campaign work is rewiring a flag's `os.Getenv`
   reader to a `policy.json` struct / DI seam / CLI flag, then marking the row `StatusDeprecated`.
   The row (a tombstone, kept for back-compat documentation) stays in `All`, so `len(All)` is
   unchanged even though a live operator dial was eliminated. Genuine progress is invisible.

2. **It fights the completeness guard.** `go/acs/regression/flagreaders` requires every live
   `EVOLVE_*` reader on any surface to have a registry row (it can only push `len(All)` *up*).
   When cycle-5 discovered a pre-existing unregistered reader (`EVOLVE_REAP_ORPHANS`), flagreaders
   *forced* a row to be added — and to keep the build green the cycle raised `FlagCeiling` 47→48.
   The minimization ratchet and the completeness invariant are in direct opposition.

3. **Its target of 0 is unreachable.** Three rows are tagged `Cluster: "Core Infrastructure
   (never consolidate)"` (`EVOLVE_PROJECT_ROOT`, `EVOLVE_PLUGIN_ROOT`, `EVOLVE_TESTING`) —
   legitimate process configuration, not feature flags. `len(All)` can never reach 0.

A same-metric unit test (`count <= const`) also has no teeth against the cycle-5 failure mode: a
cycle can raise the const in the same diff, exactly as cycle-5 did.

## Decision

**1. The campaign metric is live operator-facing feature flags, not registry rows.**
`flagregistry.LiveFeatureFlags()` = rows that are `StatusActive` **and not** core-infrastructure
(`IsCoreInfra`, keyed on the `ClusterCoreInfra` marker). This is the count the campaign drives to
~0. Deprecating a flag removes it from this metric immediately (status flips off `Active`); a
completeness-driven row addition only counts if it is a real live operator dial. At the target,
only core-infra `Active` rows + internal/test-seam plumbing remain — i.e. zero feature dials, the
`no_feature_flags` goal. `len(All)` is now a loose completeness backstop, free to rise when
flagreaders discovers an unregistered reader.

**2. Two-layer monotonic-decrease gate.**
   - **In-tree ratchet (fast, git-independent):** `LiveFeatureFlagCeiling` const + a unit test
     (`len(LiveFeatureFlags()) <= ceiling`) in `flagregistry`. Runs in CI and every cycle's build
     test. Lowering the const is progress.
   - **ACS baseline guard (the teeth):** `go/acs/regression/flagceiling` fails the per-cycle gate
     if the live count *rose* versus the campaign baseline (`origin/main`, else `main`), counting
     the baseline from the registry source on that ref. Because the baseline comes from git
     history, a cycle cannot grant itself headroom by editing in-tree files — closing the
     const-bump hole. **Fail-open** when no baseline ref is reachable (offline / shallow clone):
     the in-tree ratchet remains the floor, and CI / the full-clone cycle worktree always have the
     ref, so the teeth bite where autonomous cycles run.

**3. Core-infra is data, not a code list.** Membership is the per-row `Cluster` marker
(`ClusterCoreInfra`), reviewable in `registry_table.go`. `TestIsCoreInfra_*` pins both the marker
const against the data and the expected set against typos/renames, so the irreducible set stays an
explicit, auditable property of the registry — not a hardcoded name-list buried in gate logic.

## Consequences

- A cycle that adds a net-new live feature flag must offset it with a deprecation, or it fails the
  gate — enforcing "new behavior uses design patterns, not env dials" (no_feature_flags).
- `FlagCeiling` (row count) and `LiveFeatureFlagCeiling` (live dials) now move independently;
  comments on both spell out the split so future cycles do not re-confuse them.
- The blob line-counter in the ACS guard is pinned to `LiveFeatureFlags()` on HEAD
  (`TestBaselineCounter_AgreesWithStructOnHEAD`) so a registry row-format change fails loudly
  rather than silently miscounting the baseline.

## Alternatives rejected

- **Keep `len(All)` + a per-cycle "delete dead tombstone" step.** More literal to "FlagCeiling 0"
  but slower, still floored above 0 by core-infra, and it punishes the completeness guard.
- **A pure unit-test ratchet on the new metric.** No teeth against the const-bump failure mode
  that motivated this ADR; the git-baseline guard is the load-bearing anti-regression check.
