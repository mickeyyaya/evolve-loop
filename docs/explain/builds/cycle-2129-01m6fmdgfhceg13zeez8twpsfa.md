# Build Explanation — Cycle 2129

## Build Binding
- Cycle: 2129
- Base SHA: ca2f3ed59f6455db6a07ceb4f37ba4b448dfa451

## Summary
The worktree fence now preserves reverted writes of read-only phases. It saves `fence-reverted.patch` in the request workspace and names it in the diagnostic.

## Rationale
Read-only phases can mutate the worktree during probing or execution. Restoring the dispatched tree without preserving those mutations causes lost work. Saving the diff from the first snapshot into the workspace lets operators recover reverted writes with `git apply`.

## Changed Areas
- `go/internal/treefence/fence.go` — adds `WithWorkspace` to `Fence`, generates `fence-reverted.patch` by `git diff-tree` against the snapshot tree, and names the patch in the diagnostic.
- `go/internal/phases/runner/worktree_fence.go` — passes `req.Workspace` into `treefence.Begin` with `WithWorkspace`.

## Design Decisions
- `git diff-tree -p --binary` filters by reverted paths. Allowed writable paths stay out of the patch.
- If no writes were reverted, or if no workspace is provided, no patch file is created.
- Unwritable workspace locations emit a diagnostic indicating the patch save failure without failing the worktree restore itself.

## Verification
- Tests in `go/internal/treefence` verify patch creation, binary roundtrip, and path filtering. Clean runs and unwritable workspace paths are also verified.
- Phase runner tests in `go/internal/phases/runner` verify end-to-end integration with `req.Workspace`.
- ACS cycle predicates `TestC2129_001_FencedWriteLeavesPatchThatGivesTheWriteBack`, `TestC2129_002_FenceDiagnosticNamesThePatchPath`, and `TestC2129_003_NoFencedWriteWritesNoPatch` pass.

## Compatibility
The `WithWorkspace` method is additive and optional on `*Fence`. If `WithWorkspace` is not called or workspace is empty, behavior remains unchanged.

## Limitations
The patch captures the files that differed between the snapshots. This includes untracked files that git does not ignore. Ignored files are not in the patch.
