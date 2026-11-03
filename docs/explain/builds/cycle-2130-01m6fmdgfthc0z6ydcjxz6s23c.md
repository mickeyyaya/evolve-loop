# Build Explanation — Cycle 2130

## Build Binding
- Cycle: 2130
- Base SHA: ca2f3ed59f6455db6a07ceb4f37ba4b448dfa451

## Summary
The cmd/evolve test binary now clears inherited cell markers at test startup. This change lets CLI tests run inside an LLM cell.

## Rationale
CLI tests of inbox curation verbs and ship auto-confirm inspect environment variables to refuse execution inside phase cells. When tests run inside a cell, inherited markers cause 26 tests to refuse and fail. Clearing the markers through ipcenv.CellMarkers in TestMain restores host-environment semantics for tests. Explicit refusal tests that set markers intentionally continue to test refusal behavior.

## Changed Areas
- `go/cmd/evolve/main_test.go` — unsets inherited cell-marker environment variables in TestMain using ipcenv.CellMarkers.
- `go/cmd/evolve/main_cell_markers_test.go` — asserts that no cell marker stays set in the test process.
- `go/cmd/evolve/cmd_loop_runtime.go` — the loop runtime removes the tmux socket variable that it exported, so a test that runs the loop leaves no marker behind.
- `go/cmd/evolve/cmd_loop_runtime_env_test.go` — covers the removal, and covers a socket that the caller set.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the marker guard test as serial because it reads the process environment.
- `go/acs/cycle2130/predicates_test.go` — the four acceptance predicates of the cycle.
- `.evolve/evals/cmd-evolve-tests-hermetic-inside-a-cell.md` — the eval graders of the task.

## Design Decisions
Unsetting cell markers in TestMain runs before any test function executes. This design provides an isolated environment without modifying existing test logic.

## Verification
ACS predicates TestC2130_001 through TestC2130_004 pass. Inline eval graders and explicit refusal tests pass without regression.

## Compatibility
Existing CLI behavior and test assertions remain unchanged. Tests that explicitly set cell markers continue to verify refusal semantics.

## Limitations
Clearing markers affects only the in-process cmd/evolve test environment. Subprocesses inherit only the environment variables that tests provide.
