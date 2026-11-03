# Build Explanation — Cycle 2135

## Build Binding
- Cycle: 2135
- Base SHA: a06a61e1d75306d6b5978ef9d1eb5e77eed52073

## Summary
A run of `evolve agent run` now copies its model-try rows into the durable ledger `.evolve/agent-runs/llm-calls.ndjson` before the workspace is removed. Each copied row carries the run label and the profile name.

## Rationale
The run workspace and its `llm-calls.ndjson` were deleted without `--keep`, so the token spend of a standalone run was lost. The existing `llmcalls.Import` already deduplicates by call identity, so the change reuses it with a tagging step.

## Changed Areas
- `go/internal/llmcalls/store.go` — adds `ImportTagged`, which sets `run_id` and `profile` on each imported row; `Import` shares the same loop.
- `go/internal/llmcalls/apicover_named_test.go` — names `ImportTagged` in a test that checks tags and idempotence.
- `go/internal/cli/agentcmd/run.go` — calls `ImportTagged` after the launch and warns on stderr without changing the exit code; label generation adds an in-process sequence per project root.

## Design Decisions
A missing workspace ledger means no rows, so the run creates no durable file and prints no warning. An import failure prints a warning that names the ledger file. The run label receives a per-root sequence offset. Two runs in one process at one instant then get distinct labels. The first run still gets the plain label.

## Verification
The five ACS predicates of cycle 2135 pass. The `llmcalls` and `agentcmd` package tests pass, including the label-collision tests.

## Compatibility
`Import` keeps its signature and behavior. The first run of a project root in a process keeps its previous label.

## Limitations
Two separate processes that start in the same nanosecond with the same process id can still collide on a label.
