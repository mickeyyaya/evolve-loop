# Build Explanation — Cycle 1911

## Build Binding
- Cycle: 1911
- Base SHA: 90e752dee7973be1b82e10178f3d2d18c030b9b6

## Summary
A quota wall on one agy model now benches only that model group. A Gemini 4 Argon wall at a deep seat no longer benches Gemini 3.8 Flash at fast and balanced seats. The compact agy reset hint (`Resets in 148h8m53s`) now parses, is capped at 24 hours, and its line is kept as the bench evidence.

## Rationale
On 2026-10-17 one Argon 429 benched the whole agy family for 30 minutes while Flash still answered, so cycles 1907 and 1908 ran every phase on Claude Code (eval `agy-model-429-benches-whole-family`). The bench key was the family alone. A scope field on the existing bench record is the smallest change that keeps one store and one reader (`ApplyCLIHealthBench`). The rejected option was a second store for model benches: the package rule says the bench record is the one place that stores a wall. The compact parse ships in the same diff as the scope, because a 24-hour family-wide bench would be worse than the 30-minute one.

## Changed Areas
- `go/internal/clihealth/clihealth.go` — adds `Entry.Scope`, `Entry.Key`, `ModelGroup` and `Store.BenchScopedWall`; `Bench` and the composed benches key by `Entry.Key`, so scoped and family entries coexist. `ExhaustedPattern` keeps the clocked hints only.
- `go/internal/clihealth/resetparse.go` — adds the compact `Resets in <h><m><s>` hint, parsed with `time.ParseDuration`; zero and garbage durations are rejected; the hint joins the evidence-line search.
- `go/internal/clihealth/scoped_bench_test.go` — unit tests for `ModelGroup`, `Entry.Key` and `BenchScopedWall`.
- `go/internal/bridgechain/bridgechain.go` — `Escalation.Tier`, filled by `Walking.Launch`; `BenchOnEscalation` writes a scoped bench for a family in `perModelQuotaFamilies` (agy); `ApplyCLIHealthBench` applies a scoped entry only when the seat's tier maps to a model of that group.
- `go/internal/bridgechain/agy_quota_bench_test.go` — a table test of the bench key per CLI and tier, with agy-claude-tmux pinned family-wide.
- `go/cmd/evolve/cmd_clihealth.go` — `evolve clihealth list` prints and sorts by the key, so a scoped bench is visible and `clear` takes the printed key.
- `docs/architecture/packages/internal-clihealth.md` — records the scope, the key and the compact hint rule.
- `docs/architecture/packages/internal-bridgechain.md` — records when a model-group bench demotes a seat.
- `go/acs/cycle1911/predicates_test.go` — the TDD phase's predicates for this task.
- `.evolve/evals/agy-model-429-benches-whole-family.md` — the TDD phase's eval for this task.

## Design Decisions
The scope is the model group: the tier-map model with its effort variant removed (`Gemini 4 Argon (High)` → `Gemini 4 Argon`). Deep and top share the Argon group, and fast and balanced share Flash. The set `perModelQuotaFamilies` in bridgechain names the families that pool quota per model: agy only. agy-claude stays family-wide, because Claude and GPT models share one pool. A manifest flag was the first design, but `go/internal/bridge/` is a protected surface (ADR-0064), so moving the fact there is console work. A tier outside the tier map gives a family-wide bench, so an unmapped wall never writes a scope that no seat matches. A seat matches a scope through the model its tier dispatches under the loaded tier map. Write and read use the same map, so a wall and the next seat agree in one environment.

## Verification
`go test -count=1 ./internal/clihealth/ ./internal/bridgechain/ ./internal/phases/runner/ ./cmd/evolve/` passes. `go test -tags acs -count=1 ./acs/cycle1911` passes all six predicates under the runtime catalog. Predicates 003 and 006 derive the deep model group from `bridge.LoadManifest("agy-tmux")`, the same tier map that the code reads. Thus they also pass under the hub catalog, which maps agy deep to `Gemini 3.1 Pro (High)`.

## Compatibility
A legacy `cli-health.json` without `scope` reads as family-wide entries, and its keys are unchanged. Consumers that look up a family key see a scoped bench as no family bench, which is the intended result.

## Limitations
`ApplyCLIHealthBench` judges a scoped bench by the plan's first tier only. A tier step-down inside one walk does not re-order candidates. The runner's walk (`go/internal/phases/runner/dispatch.go`, protected) and `usageevidence` send escalations without a tier, so their walls stay family-wide. A scoped bench follows the live catalog: after a catalog change, an older scope can match no seat until it expires.
