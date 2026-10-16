# Build Explanation — Cycle 1889

## Build Binding
- Cycle: 1889
- Base SHA: 1e20b75e04922001b149e73d7171f6c99918ce5a

## Summary
A failed boundary step now keeps its last 40 stderr lines. The `evolve wave next --json` envelope puts them in the step's `detail` after `rc=N`. The boundary log prints them after the `failed rc=N` line. `evolve doctor live` now ends each failure report with one `[doctor] failing stage: <stage>` line. The stage is launch, trust prompt, REPL prompt, delivery or artifact.

## Rationale
At the wave 101 and 102 boundaries (2026-10-16), the first `doctor live claude-tmux` step failed with rc=1. The envelope kept only `rc=1`, so the cause was lost. The step's stderr already went to the caller's stderr. A tee into a small per-step buffer keeps that text without a change to the dispatch signature. A stage name in the doctor output tells the reader where the probe stopped. The stage line is the last line of the report, so it is always inside the 40-line tail.

## Changed Areas
- `go/cmd/evolve/cmd_boundary.go` — adds `stderrTail`, a writer that keeps the last 40 lines (each clipped to 4096 bytes), and `dispatchKeepingStderrTail`, which tees one step's stderr into a new tail. `runBoundarySteps` writes the failed step's tail after the failure line.
- `go/cmd/evolve/cmd_wave_next.go` — `waveStepRecorder.dispatch` keeps a tail for each step; `waveStepDetail` adds it to `Detail` only when rc is not 0, so passing steps stay `rc=0`.
- `go/internal/cli/opscmd/doctor_live.go` — adds `liveFailingStage`, which maps the bridge exit code and a trust dialog in the pane tail to one stage, and prints the stage at the end of the WRONG MODEL, WALLED and FAILED reports.
- `go/cmd/evolve/cmd_boundary_tail_test.go` — edge cases of the tail buffer: split writes, an unterminated line, empty lines, the line bound and the size bound.
- `go/internal/cli/opscmd/doctor_live_failing_stage_test.go` — the stage map for each exit code and for the claude, codex and agy trust dialogs.
- `go/cmd/evolve/cmd_wave_test.go` — the failed sync-main step's detail now carries its stderr line after `rc=7`.
- `go/internal/cli/opscmd/doctor_report_characterization_test.go` — the pinned doctor live reports now end with their failing stage.
- `go/cmd/evolve/cmd_wave_step_tail_test.go` — TDD-phase tests for the envelope tail and the boundary log tail (unchanged by the build).
- `go/internal/cli/opscmd/doctor_live_stage_test.go` — TDD-phase tests for the stage line (unchanged by the build).
- `go/acs/cycle1889/predicates_test.go` — TDD-phase ACS predicates of this cycle (unchanged by the build).
- `.evolve/evals/boundary-doctor-live-keeps-no-output.md` — the eval of this task (unchanged by the build).

## Design Decisions
The tail buffer is one type in `cmd_boundary.go`, and both the boundary runner and the wave recorder use it. Thus the two outputs cannot drift. Each step gets a new buffer, so a failed step's tail never holds an earlier step's stderr. The buffer drops old lines on each write, so its memory stays bounded for a long stream. A trust dialog in the last 12 pane lines wins over the exit code, but a launch failure wins over both, because no pane exists before launch. Exit codes 85, 86, 124 and other codes map to delivery. Rejected option: a stage field in the bridge result. The bridge returns only an exit code here, and that change has a much larger blast radius.

## Verification
`go test -count=1 ./cmd/evolve ./internal/cli/opscmd` passes. `evolve acs suite --cycle 1889` gives green=171 red=0. The four predicates of cycle 1889 are green.

## Compatibility
The envelope keeps its schema; only the `detail` text of a failed step grows. Passing steps keep `rc=0`. The doctor live JSON payload does not change; the stage line goes to stderr only.

## Limitations
The fix does not reproduce the first-attempt rc=1 failure of `doctor live claude-tmux`. A unit test cannot drive a live tmux session. The next failing boundary now keeps the output that a diagnosis needs.
