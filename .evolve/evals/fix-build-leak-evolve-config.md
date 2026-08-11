# Eval: fix-build-leak-evolve-config

## Task
Narrow the `.evolve/` skip in `recoverBuildLeak` so tracked config files
(e.g. `.evolve/commit-prefix-scope.json`) are relocated into the worktree
rather than being silently left as main-tree leaks that trip the tree-diff guard.

## Acceptance Criteria

### 1. [code] Tracked config file is relocated, not skipped

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/core/... -run TestRecoverBuildLeak_EvolveConfigFile -v
```

Expected: `PASS` — a simulated build that modifies `.evolve/commit-prefix-scope.json`
in the main tree has the file relocated to the worktree, not left as a leak.

### 2. [code] Untracked runtime state is still skipped

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/core/... -run TestRecoverBuildLeak_EvolveRuntimeSkip -v
```

Expected: `PASS` — untracked `.evolve/guards.log` and `.evolve/runs/cycle-N/...`
files are still skipped (not relocated), preserving the cycle-176 fix.

### 3. [code] Full test suite passes

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/core/... 2>&1 | tail -3
```

Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/core` (all tests pass).

### 4. [code] No regression in phaseoutcome test

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/core/... -run TestPhaseOutcome_TreeGuardAbort -v
```

Expected: `PASS` — existing tree-guard abort test still passes (cycle still aborts
if the .evolve/ file cannot be relocated for another reason).
