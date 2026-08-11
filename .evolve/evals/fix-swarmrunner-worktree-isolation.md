# Eval: fix-swarmrunner-worktree-isolation

Phase: builder
Cycle: 293

## Description
Fix `TestDecorator_EnforceWriter_WorkerFailureReturnsFail` so that running the swarmrunner test suite never creates git worktrees or `.evolve/` directories inside the source package directory. Add a defensive guard to `provision.go::worktreeBase` to prevent recurrence.

## Acceptance Criteria

### C1: No residue in swarmrunner package dir [code]
After running `go test ./internal/phases/swarmrunner/... -count=1` (from `go/`), the path `go/internal/phases/swarmrunner/.evolve/` MUST NOT exist.

Command:
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/swarmrunner/... -count=1 && \
  test ! -d internal/phases/swarmrunner/.evolve && echo "PASS: no residue" || echo "FAIL: residue found"
```

### C2: No orphaned worktrees in git registry [code]
After running the test suite, `git worktree list` MUST NOT list any paths under `go/internal/phases/swarmrunner/`.

Command:
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  git worktree list | grep -c "swarmrunner" && echo "FAIL: orphaned worktrees" || echo "PASS: no swarmrunner worktrees"
```

Expected: `git worktree list | grep swarmrunner` exits non-zero (no matches).

### C3: Suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... ./internal/phases/swarmrunner/... -count=1
```
Both packages exit 0.

### C4: Defensive guard present [code]
`worktreeBase` in `provision.go` must either (a) call `filepath.Abs` on the result when `EVOLVE_WORKTREE_BASE` is unset, OR a post-suite assertion in `swarmrunner_test.go` (`TestMain` or `t.Cleanup`) verifies no `.evolve/` dir exists under the package. Verify via:
```bash
grep -n "filepath.Abs\|TestMain\|.evolve" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/swarm/provision.go \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/swarmrunner/swarmrunner_test.go
```

## Negative Cases

### N1: Test with a gitInit repo + pinned base still passes [code]
The new test helper `gitInitForSwarm(t)` must produce a valid git repo in TempDir that satisfies `git worktree add`. No source-tree operations.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/swarmrunner/... -run TestDecorator_EnforceWriter_WorkerFailureReturnsFail -v -count=1
```
Must pass AND leave zero residue under `go/internal/phases/swarmrunner/`.

### N2: Old relative-ProjectRoot path is blocked [model]
After the guard is in place, if `ProjectRoot: "."` is used in a writer-mode test WITHOUT setting `EVOLVE_WORKTREE_BASE`, the resulting path from `worktreeBase` should be absolute (resolved via `filepath.Abs`). This converts the relative "." to the test's actual CWD, making the problem visible in stack traces but no longer silently polluting the source tree (since `EVOLVE_WORKTREE_BASE` is now always set in writer tests).
