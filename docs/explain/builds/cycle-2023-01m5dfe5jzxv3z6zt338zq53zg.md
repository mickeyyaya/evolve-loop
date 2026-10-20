# Build Explanation — Cycle 2023

## Build Binding
- Cycle: 2023
- Base SHA: f7b28ac26df8076e89a0f6d8d557fb8fcbf4327f

## Summary
`evolve cycle models --cycle N` is a new read-only verb. It prints one row per model attempt of a cycle: phase, attempt, CLI, tier, selector and the model that served the call. A record without a served model prints `unreported`. A cycle with no call records exits 1 and says why on stderr.

## Rationale
An operator learned which model served a phase by reading CLI transcripts by hand. The bridge already writes `dispatched_model` and `served_model` on each `llm-calls.ndjson` line (`llmcalls.Record`), so the missing part was a reader. The verb reads the ledger through `llmcalls.ReadWorkspace` and adds no new store.

## Changed Areas
- `go/cmd/evolve/cmd_cycle_models.go` — adds `runCycleModels`, the row projection, the table renderer and the `--json` output.
- `go/cmd/evolve/cmd_cycle.go` — dispatches `models` and lists it in the missing-subcommand usage.
- `go/cmd/evolve/cmd_cycle_models_test.go` — unit tests of the table, the `unreported` fallback, one row per attempt, the empty-cycle exit, `--json` and the dispatch.
- `go/acs/cycle2023/predicates_test.go` — acceptance predicates that run the built binary on fixture ledgers.
- `.evolve/evals/cli-cycle-models-lane.md` — the eval for this task.

## Design Decisions
The selector column is `Record.ModelIdentity()`, not the raw field, so a legacy line prints `(unknown)` and never a fabricated model. The tier column is `Record.Model`, the tier alias. Rejected: a second recorder for the served model, because acceptance 2 holds through the existing bridge `TryRecord` tests (`TestTryRecord_ServedModel*`). Without `--cycle`, the verb picks the latest cycle that has an `llm-calls.ndjson`, like `cycle timing`. The loader is split from the command so each function stays within the 50-line ratchet.

## Verification
`go test -tags acs -count=1 ./acs/cycle2023` passes (4/4). `go test -count=1 -run 'TestRunCycleModels|TestRunCycle_Models|TestRatchet_Check' ./cmd/evolve` passes. `evolve acs suite --cycle 2023` reports green=239 red=0.

## Compatibility
No existing flag, field or file changes. The verb is additive and read-only.

## Limitations
Only the model the CLI reports is shown. A launch that reports none prints `unreported`; the verb does not guess it from the selector.
