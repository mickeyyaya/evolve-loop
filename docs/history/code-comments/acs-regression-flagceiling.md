# Comment history: `acs/regression/flagceiling`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/flagceiling/ratchet_test.go:3` — above `package flagceiling`

```text
// Package flagceiling is the ACS monotonic-decrease guard for the flag-reduction
// campaign. It fails the per-cycle gate if the count of live operator-facing
// feature flags (flagregistry.LiveFeatureFlags = StatusActive minus
// core-infrastructure) ROSE versus the campaign baseline (main).
//
// Why a guard and not just the LiveFeatureFlagCeiling unit ratchet: a same-metric
// unit test (count <= const) can be defeated by editing the const in the same
// diff — exactly how cycle-5 raised FlagCeiling 47->48 to absorb a net addition.
// This guard derives the baseline from git history (origin/main, else main), so a
// cycle cannot grant itself headroom by editing in-tree files.
//
// Fail-OPEN when no baseline ref is reachable (offline / shallow clone): the
// in-tree LiveFeatureFlagCeiling ratchet remains the floor and CI / the per-cycle
// worktree (full clone) always have the ref, so the teeth bite where it matters.
```
