# Build Explanation — Cycle 2160

## Build Binding
- Cycle: 2160
- Base SHA: e69d7f45bb52d67e4150fd401a58d66dd5390717

## Summary
An exhausted per-model usage window now benches only its model. The claude Opus week benches the pair claude and Opus. The claude family keeps its place in the chain.

## Rationale
The weekly Opus window drains before the family does. A family bench stops Sonnet work that still has quota. A model bench demotes claude only where the seat resolves to Opus. The manifest holds the word-to-model map, so Go holds no list of model names.

## Changed Areas
- `go/internal/bridge/manifests/claude-tmux.json` — adds `model_match`, which maps the scope word Opus to the pattern for Opus models.
- `go/internal/quotastate/windows.go` — adds the `ModelMatch` field, the `Unmapped` window flag and `ModelMatches`, and validates the patterns when the spec compiles.
- `go/internal/bridge/usage_model.go` — adds `UsageModelMatches`, `UsageModelFor` and `UsageModelMapped`, which read the manifest map for a driver.
- `go/internal/clihealth/clihealth.go` — adds `ClearModel` and `ExpiredModels` so the canary can find and clear a model bench.
- `go/internal/bridgechain/bridgechain.go` — splits the family bench from a new model bench. The model bench demotes a candidate only when its tier model matches.
- `go/cmd/evolve/cli_health_model_canary.go` — probes each expired model bench with its model, clears it on success, re-benches it on a wall and keeps it on a model-mismatch exit.
- `go/cmd/evolve/cli_usage_probe.go` — runs the model canary in the pre-wave probes, after the family canary, and wires the manifest map into the usage prober.
- `go/internal/usageprobe/usageprobe.go` — adds the `ModelMapped` seam to the prober.
- `go/internal/usageprobe/windows.go` — an exhausted model window whose scope word the manifest does not map keeps the WARN and writes no bench row.
- `go/internal/usageevidence/production.go` — the window reader flags a model window that the manifest does not map. The evidence path then benches nothing.
- `go/internal/usageevidence/model_unmapped_test.go` — tests the evidence path and a bare prober: an unmapped word benches nothing, a mapped word still benches its model.
- `go/internal/usageprobe/model_mapped_test.go` — tests that the prober benches only mapped words.
- `go/cmd/evolve/cli_usage_probe_model_mapped_test.go` — tests the production prober with the claude manifest for Opus and Fable.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the pre-wave model test as serial because it swaps a package seam.
- `go/internal/repocontract/repocontract.go` — adds the new pre-wave production-prober test to the tree-reading selection, so ship's repo-contract pack runs it with the other tests that read the tree.
- `go/pkg/acsassert/predicate_kit.go` — makes `ModuleDir` resolve a relative start to an absolute path, so a start of `.` climbs to `go.mod`.
- `go/internal/bridge/usage_model_test.go` — names and checks `UsageModelMatches` and `UsageModelFor`. `UsageModelMapped` has no direct test here. The production-prober tests in `go/cmd/evolve/cli_usage_probe_model_mapped_test.go` reach it.
- `go/cmd/evolve/cli_health_model_canary_test.go` — tests the canary for a model bench.
- `go/internal/bridgechain/model_bench_demotion_test.go` — tests the deep and balanced chains under an Opus bench.
- `go/internal/clihealth/model_canary_store_test.go` — tests the store verbs for model benches.
- `go/internal/quotastate/model_match_test.go` — tests the model-match rules.
- `go/acs/cycle2160/predicates_test.go` — holds the ten acceptance predicates of the task.
- `.evolve/evals/model-level-benches-from-per-model-usage-windows-console.md` — holds the eval graders of the task.

## Design Decisions
The map is data in the manifest. A scope word that the map does not list benches nothing and keeps the WARN. A bench stores the scope word, and the demotion resolves it to a model per candidate at plan time. The alternative, a Go table of word to model, was rejected because it duplicates manifest data.

## Verification
The ACS suite for cycle 2160 reports green=273 red=0. The touched packages and `cmd/evolve` pass with `-count=1`.

## Compatibility
Family benches work as before. Manifests without `model_match` bench no model.

## Limitations
Only the claude manifest maps a word today. Other drivers need their own `model_match` entry.
