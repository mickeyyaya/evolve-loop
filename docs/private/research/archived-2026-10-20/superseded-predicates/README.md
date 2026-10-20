# Retired ACS predicates: the retro model contract changed (2026-10-20)

> Archived by the console lane for the inbox item `retro-events-stream-missing` ([the package note](../../../../architecture/packages/internal-phases-retro.md)). The retrospective now dispatches through the shared phase runner, so its model resolution is the one of every runner phase. The predicate below pinned the old retro-only rule, and the new contract reverses it. It is archived here under the A2 retention rule (never deleted, never silently edited; precedent: [the 2026-10-18 archive](../../archived-2026-10-18/superseded-predicates/README.md) and the ledger in [acs-predicate-quality-gate.md](../../../../architecture/acs-predicate-quality-gate.md)). The rest of `go/acs/cycle1568` stays live.

## What was archived and why

| Predicate | It asserted | Why it is retired | What pins the contract now |
|---|---|---|---|
| `TestC1568_005_explicit_model_tier_unchanged` | An explicit `Config.Model` tier reaches the bridge unchanged, even when the `retrospective` profile names a different `model_tier_default`. | The shared runner resolves the model through `llmroute.Resolve`: the env tier, then the profile tier, then the default of the hooks. Thus `Config.Model` is now the default below the profile tier, as it is for every runner phase. Every production caller passes `auto` (`go/cmd/evolve/cmd_cycle.go`, the registry factory in `go/internal/phases/retro/retro.go`), so no live dispatch depended on the old rule. | `internal/phases/retro` `TestRun_TheConfiguredModelIsTheDefaultBelowTheProfileTier` |

## What changed in place

| Predicate | Action | Now |
|---|---|---|
| `TestC1568_006_sentinel_never_survives_degraded_resolution` | repointed | Its first name was `TestRun_AutoModel_ProfileWithoutTier_ResolvesToDefaultNotAuto`. The successor `TestRun_AutoModel_ProfileWithoutTier_ResolvesThroughTheRouterNotAuto` pins the same scenario: a profile with no tier never dispatches the `auto` sentinel. The model now comes from the root router, not from the literal `balanced`. The second name, `TestRun_AutoModel_NoProfile_NeverDispatchesSentinel`, is unchanged. |

## Notes

- The archived file keeps the package clause, the helpers and the body of the predicate, so it reads as it ran. It lives outside the Go module and never builds.
- `go/acs/regression/acsnames` reads `go/acs` only. It does not read this archive, which holds the stale name.
