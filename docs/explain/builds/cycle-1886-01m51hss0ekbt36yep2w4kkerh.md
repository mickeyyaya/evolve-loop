# Build Explanation — Cycle 1886

## Build Binding
- Cycle: 1886
- Base SHA: cdc0a4083bb2788518b371f4ed7b85721bdc0b85

## Summary
`evolve loop` records the goal text of each real launch in `.evolve/last-goal.txt`. The new flag `--goal-from-last` replays that record. The new flag `--goal-file F` reads the goal from a file. The dry-run config names the goal source in `goal_source`.

## Rationale
Operators typed each wave goal by hand, and a quoting slip changed the goal hash without a warning. A recorded goal and a goal file remove the retyped text. The existing `goalhash.Compute` path hashes the goal, so a replayed goal gets the same hash.

## Changed Areas
- `go/cmd/evolve/cmd_loop_goal_source.go` — new file. It counts the goal sources, refuses a conflict with exit 10, reads the goal file (exit 10 when missing or blank) and the record (exit 1 when absent or blank), names the source, and writes the record through `atomicwrite.Bytes`.
- `go/cmd/evolve/cmd_loop_args.go` — registers `--goal-file` and `--goal-from-last`, resolves the evolve directory before the goal so that the record path is known, and sets `GoalSource` on the config.
- `go/cmd/evolve/cmd_loop.go` — adds the `goal_source` field to `loopConfig` and records the goal in `runLoop` after the detach branch, so only the process that runs the batch writes it.
- `docs/operations/runtime-reference.md` — the launch step documents both flags, the record, the exit codes and `goal_source`.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the eight new goal-source tests as serial. The two launch tests swap `wireOrchestratorDepsFn`. The other six do not call `t.Parallel`, and the TDD phase owns their file.
- `go/cmd/evolve/cmd_loop_goal_source_test.go` — the TDD phase unit tests for the goal-source contract.
- `go/acs/cycle1886/predicates_test.go` — the TDD phase ACS predicates for this cycle.
- `.evolve/evals/cli-loop-goal-from-last.md` — the TDD phase eval for this task.

## Design Decisions
The two new flags are exclusive: each refuses every other goal source. The old pair `--goal-hash` with `--goal-text` keeps its behavior, because it is out of scope. A `--dry-run` records nothing, so a plan cannot replace the goal of the last real launch. A failed record write prints a WARN and does not stop the launch, because the record is an operator convenience and not a gate. Rejected option: write the record in `runLoopBatch`. A chain launch and a re-exec handoff take other paths, so one write in `runLoop` covers all of them.

## Verification
`go test -count=1 -run '^TestLoopGoalSource_' ./cmd/evolve` passes. `go test -tags acs -count=1 ./acs/cycle1886` passes all seven predicates.

## Compatibility
The existing flags and their exit codes do not change. The `goal_source` JSON field is new and is omitted when no goal source is set, for example on `--resume`.

## Limitations
The record keeps one goal only. It has no history and no per-wave index.
