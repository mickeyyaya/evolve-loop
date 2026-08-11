# Eval: swarm-worktreebase-guard

## Goal
`worktreeBase()` in `go/internal/swarm/provision.go` must itself refuse a relative result and return `(string, error)`, so the guard fires at the function boundary rather than only inside `addWorktree`.

## Criteria

### C1 — worktreeBase returns error on relative EVOLVE_WORKTREE_BASE [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... -run "TestWorktreeBase" -count=1 -v 2>&1
```
**Expected:** All TestWorktreeBase_* pass. `TestWorktreeBase_EnvOverride` and `TestWorktreeBase_DefaultPath` compile and pass with the new `(string, error)` signature. A test for relative EVOLVE_WORKTREE_BASE returns an error from `worktreeBase` itself (not only from `addWorktree`).

### C2 — full swarm suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... -count=1 2>&1
```
**Expected:** `ok github.com/mickeyyaya/evolve-loop/go/internal/swarm` — all tests PASS, zero failures.

### C3 — negative: truly-relative path triggers error at worktreeBase call site [code]
The `TestAddWorktree_RelativeBaseRefused` test (or equivalent) must verify the error message mentions "absolute" and comes from the `worktreeBase` guard, and that `os.Stat` on the relative path returns `IsNotExist` (no filesystem side effects).
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... -run "TestAddWorktree_RelativeBaseRefused|TestGitProvisioner_RelativeWorktreeBase" -count=1 -v 2>&1
```
**Expected:** Both tests PASS with "absolute" in the error message and no side-effect directory created.

### C4 — addWorktree no longer contains duplicate IsAbs check [code]
```bash
grep -n "IsAbs" /Users/danleemh/ai/claude/evolve-loop/go/internal/swarm/provision.go
```
**Expected:** Only ONE occurrence of `filepath.IsAbs` in the file — inside `worktreeBase`, not additionally in `addWorktree`.

### C5 — swarmrunner suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/swarmrunner/... -count=1 2>&1
```
**Expected:** `ok github.com/mickeyyaya/evolve-loop/go/internal/phases/swarmrunner` — all tests PASS.
