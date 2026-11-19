# Build Explanation — Cycle 2538

## Build Binding
- Cycle: 2538
- Base SHA: f31324cca92aa50ea853c803fe43e130f457bd05

## Summary
`evolve agent run` now keeps `fence-reverted.patch` beside the launch record before workspace removal and prints the kept path in `WARN` and JSON diagnostics.

## Rationale
Moving only the reverted patch to `.evolve/agent-runs/<run_label>/fence-reverted.patch` preserves reverted work while keeping `.evolve/ephemeral/agent-runs/` clean after every run.

## Changed Areas
- `docs/architecture/packages/internal-cli-agentcmd.md` — documents the `keepFencePatch` relocation and diagnostic rewrite under the fence-revert section.
- `go/internal/cli/agentcmd/run.go` — calls `keepFencePatch` from `c.run` before deferred workspace removal to move `fence-reverted.patch` beside the launch record.
- `go/internal/cli/agentcmd/verdict.go` — adds `launchOutcome.withRevertedPatchPath` to rewrite the fence-restore warning diagnostic to the kept patch path.

## Design Decisions
Relocating `fence-reverted.patch` inside `agentcmd` leaves `treefence`, `core.PhaseRequest`, and `runner.go` unchanged. Placing the file under `.evolve/agent-runs/<run_label>/` keeps the basename `fence-reverted.patch`, isolates concurrent runs, and stays ignored by `launchrecord.List`.

## Verification
All three `acs/cycle2538` predicates and the `go/internal/cli/agentcmd` unit suite pass after the final edit, along with `gofmt`, `go vet`, and `evolve docs ste-lint --strict`.

## Compatibility
Runs without fence reverts leave no patch file, create no patch directory, and still remove their ephemeral workspace.

## Limitations
The `[runner]` diagnostic log line emitted during `runner.Run` still prints the first workspace path because `runner` executes before `agentcmd` relocates the file.
