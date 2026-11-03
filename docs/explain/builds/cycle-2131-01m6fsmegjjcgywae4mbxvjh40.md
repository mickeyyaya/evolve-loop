# Build Explanation — Cycle 2131

## Build Binding
- Cycle: 2131
- Base SHA: 7ecdd432b587e48873d384230de3cf4630f79540

## Summary
`evolve agent run` now refuses a `--writable` that names a directory. The refusal exits with the usage code and starts no seat.

## Rationale
The runner keeps only the exact files that `--writable` names. A directory path keeps no change, so an early refusal is better than a silent loss.

## Changed Areas
- `go/internal/cli/agentcmd/profiles.go` — `admitWriter` stats each `--writable` path in the worktree and refuses a directory.
- `go/internal/cli/agentcmd/agent.go` — the usage text states that a `--writable` names a file, never a directory.
- `go/internal/cli/agentcmd/agent_writable_test.go` — tests for the refusal, the file and new-file cases, and the usage line.
- `docs/architecture/agent-run.md` — the contract states the directory rule.
- `go/acs/cycle2131/predicates_test.go` — the acceptance predicates of the cycle.
- `.evolve/evals/agent-run-refuses-a-directory-writable.md` — the eval of the task.

## Design Decisions
The check lives in the existing `admitWriter` loop, so it shares the usage exit code and the existing refusals. A path that does not exist passes, so a new file still works.

## Verification
`go test -count=1 ./internal/cli/agentcmd` passes, and the ACS suite for cycle 2131 is green.

## Compatibility
A run that names a file, or a file that does not exist yet, behaves as before.

## Limitations
The check runs on the file system state at launch. A path that becomes a directory later is not checked.
