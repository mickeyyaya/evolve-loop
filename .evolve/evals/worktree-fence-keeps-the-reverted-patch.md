---
score_cap:
  - criterion: "A fenced write leaves fence-reverted.patch in the workspace and git apply gives the write back"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestEnd_FencedWritesLeaveAPatchThatGivesThemBack|TestEnd_PatchHoldsOnlyTheRevertedPaths)$' ./internal/treefence"
  - criterion: "The runner passes the request workspace to the fence so a fenced phase leaves the patch"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRun_FencedWriteLeavesAPatchInTheWorkspaceThatGivesItBack$' ./internal/phases/runner"
  - criterion: "The fence diagnostic names the patch path"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRun_FenceDiagnosticNamesThePatchPath$' ./internal/phases/runner"
  - criterion: "A run with no fenced write writes no patch file"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestEnd_NoFencedWriteWritesNoPatch$' ./internal/treefence"
---

# Eval: The worktree fence keeps the reverted patch

> The worktree fence restores each write that a read-only phase makes to its
> worktree. Before this task the restore discarded the write with no copy.
> This eval pins the contract that the fence saves `fence-reverted.patch` in the
> workspace, names it in its diagnostic, and writes nothing when no write was
> fenced. Source incident: an H9 console-builder run on 2026-11-03 lost ten
> written files to the fence (cycle 2129, inbox item
> `worktree-fence-keeps-the-reverted-patch`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| patch-round-trip | `git apply` of the patch gives back the reverted write | 4/10 | `go test -run TestEnd_FencedWritesLeaveAPatchThatGivesThemBack ./internal/treefence` |
| runner-wiring | the production caller passes `req.Workspace` | 4/10 | `go test -run TestRun_FencedWriteLeavesAPatchInTheWorkspaceThatGivesItBack ./internal/phases/runner` |
| diagnostic-names-path | the diagnostic names the patch path | 6/10 | `go test -run TestRun_FenceDiagnosticNamesThePatchPath ./internal/phases/runner` |
| clean-run-no-patch | no fenced write, no patch file | 6/10 | `go test -run TestEnd_NoFencedWriteWritesNoPatch ./internal/treefence` |
