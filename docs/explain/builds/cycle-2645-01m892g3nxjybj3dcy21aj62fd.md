# Build Explanation — Cycle 2645

## Build Binding
- Cycle: 2645
- Base SHA: d36bdd511d4734ff28dd606601e42b42b99a2f9b

## Summary
The `evolve gc --only salvage` command now thins full bundles whose commits left the hub repository and enforces `gc.salvage_max_mb` as a landed-only size cap.

## Rationale
The existing repack refused a full bundle when its head commit left the repository, and `.evolve/operator-salvage` had no size cap. Fetching the self-contained bundle into a scratch repository lets `origin/main` thin it without temporary hub refs. Restricting eviction to landed leaves with no patch, no non-empty untracked archive, and no hold bounds store size without data loss.

## Changed Areas
- `go/internal/gcpolicy/gcpolicy.go` — adds `SalvageMaxMB` (`salvage_max_mb`) and `SalvageMaxBytes` so zero or unset means no cap.
- `go/internal/gc/salvage_repack.go` — shares a scratch bare repository helper and fetches missing full-bundle heads into scratch. This lets `PlanSalvageRepack` and `buildThinBundle` thin legacy bundles whose commits left the hub.
- `go/internal/gc/salvage_retention.go` — adds `PlanSalvageRetention` and `ApplySalvageRetention`. They sort eligible landed leaves oldest first, stop at the cap, and re-check under the ship lock before removal.
- `go/cmd/evolve/cmd_gc_scope.go` — runs salvage retention inside `salvageScope` for both dry-run previews and real runs.
- `docs/architecture/packages/internal-gc.md` — documents full-bundle scratch migration and `gc.salvage_max_mb` retention rules beside `salvage_ttl_days`.
- `docs/operations/runtime-reference.md` — adds `salvage_max_mb` to the `gc` policy knobs reference.

## Design Decisions
Full-bundle migration uses a temporary bare repository with `objects/info/alternates` to the hub instead of temporary hub refs. Failed or concurrent runs leave hub refs untouched. Size-cap retention runs inside `salvageScope` instead of `gc.Plan` because landing checks require git access to `origin/main`. A zero or absent `salvage_max_mb` disables eviction so existing deployments keep all leaves until configured.

## Verification
All eight cycle-2645 ACS predicates pass through the built `evolve` binary, along with targeted unit tests in `internal/gcpolicy`, `internal/gc`, and `cmd/evolve`.

## Compatibility
Existing `gc` policy files without `salvage_max_mb` behave identically because zero disables the cap. The `salvage_ttl_days` rule in `gc.Plan` remains unchanged.

## Limitations
An unlanded, held, patched, or untracked-bearing leaf stays even when it alone holds the store over `gc.salvage_max_mb`. `salvageScope` prints a warning when the store stays above the cap.
