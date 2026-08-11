# Eval: tree-diff-leak-fix

## Objective
Verify that the build phase no longer writes `.evolve/commit-prefix-scope.json` (or any other path outside its worktree) to the main repository tree during normal cycle execution.

## Acceptance Criteria

### Criterion 1: commitprefixgate path resolution is worktree-relative [code]
```bash
# The ManifestPath must default to the worktree's .evolve dir, not main repo root.
grep -n "ManifestPath\|RepoDir\|WorktreeDir\|worktree" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/commitprefixgate/commitprefixgate.go | head -20
```
Expected: `ManifestPath` is set relative to the phase's working directory (worktree path), not a hardcoded `.evolve/` off the project root.

### Criterion 2: No main-tree write during build phase simulation [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/commitprefixgate/... -v -run TestManifestPath 2>&1 | tail -20
```
Expected: PASS — test confirms ManifestPath respects the provided RepoDir argument.

### Criterion 3: Go test suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/commitprefixgate/... 2>&1 | tail -5
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/commitprefixgate` — no regression.

### Criterion 4 (negative): Attempting to write to main tree while worktree path is set must fail or be redirected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/commitprefixgate/... -run TestWorktreeBoundary -v 2>&1 | tail -10
```
Expected: test asserts that Gate.ManifestPath under a worktree root does NOT equal the main-repo `.evolve/commit-prefix-scope.json`.
