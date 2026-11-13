# Build Explanation — Cycle 2357

## Build Binding
- Cycle: 2357
- Base SHA: 722ecf0d20a8e61948b4a9bdc817ac90d4672b4d

## Summary
A writer seat can now be granted a directory for new files. A new file outside every grant is still removed, and the seat gets an error diagnostic that names the path and the grant to add.

## Rationale
The fence removed a file that a seat created outside its `--writable` list. It logged a warning only, so the seat verdict stayed PASS for work that the tree no longer held. A directory grant lets a seat add files without naming each one. The error diagnostic makes the removal visible in the envelope, and `agent run` ends such a seat with verdict FAIL.

## Changed Areas
- `go/internal/treefence/fence.go` — adds `WithWritableDirs`, `Result.Created`, `Outcome.Created`, `Outcome.Writer` and the error diagnostic. A new file under a granted directory is kept and verified.
- `go/internal/treefence/apicover_named_test.go` — names the new exports in a real assertion.
- `go/internal/core/worktree_fence.go` — adds the `writable_dirs` field, `WritableDirPaths` and `GrantFence`.
- `go/internal/core/writes_source.go` — a request with only a directory grant is a writer.
- `go/internal/core/worktree_fence_grant_test.go` — tests the grant accessors and the writer rule.
- `go/internal/phases/runner/worktree_fence.go` — passes the directory grant from the request to the fence.
- `go/internal/cli/agentcmd/run.go` — adds the `--writable-dir` flag and builds the fence from both grants.
- `go/internal/cli/agentcmd/verdict.go` — adds `withFenceVerdict`. A PASS seat with an error diagnostic ends FAIL, so the exit code and the launch record agree.
- `go/internal/cli/agentcmd/profiles.go` — refuses an unclean directory, a file, and a reader that gets a grant.
- `go/internal/cli/agentcmd/agent.go` — states the directory grant and the error in the usage.
- `docs/architecture/packages/internal-treefence.md` — documents the directory grant and the error.

## Design Decisions
The directory grant covers new files only, so an existing file outside the file grant is still restored. The verdict downgrade lives in `agent run`, the only caller that grants a fence to a writer seat. The runner and verdict engine are protected paths and stay unchanged.

## Verification
The cycle 2357 predicates and the agentcmd, treefence and core tests pass. The ACS suite result is in build-report.md.

## Compatibility
A request without `writable_dirs` behaves as before. A read-only phase keeps its warning.

## Limitations
A phase run through the runner, not `agent run`, keeps its verdict. No such phase grants a fence to a writer today.
