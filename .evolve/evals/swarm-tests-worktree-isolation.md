# Eval: swarm-tests-worktree-isolation

## Task
Fix swarm and swarmrunner tests to never materialise git worktrees inside the package directory. Every test that exercises the real `gitWorkerProvisioner` must pin `EVOLVE_WORKTREE_BASE` to a `t.TempDir()` path, and the root used for `git worktree add` must be a throwaway git repo (not `.`).

---

## Criterion 1 — Stale worktrees cleaned up [code]

```bash
# Must produce no output (no stale worktrees registered from the package dir)
git worktree list | grep "go/internal/phases/swarmrunner/.evolve" && echo FAIL || echo PASS
```

Expected: `PASS`

Gaming fake: a script that removes the worktrees without fixing the test that creates them → next test run recreates them.
Negative check: before the fix, this prints the 3 stale worktrees and exits 0.

---

## Criterion 2 — No .evolve/worktrees inside the swarmrunner package dir after `go test` [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/swarmrunner/... -count=1 -timeout 60s 2>&1 | tail -3 && \
  ls go/internal/phases/swarmrunner/.evolve/worktrees 2>/dev/null | wc -l | \
    xargs -I{} test {} -eq 0 && echo PASS || echo FAIL
```

Expected: tests pass AND the directory is empty.

Gaming fake: deleting the dir after the test without fixing the test code.

---

## Criterion 3 — dispatcher_test.go contains no `ProjectRoot: "."` [code]

```bash
grep 'ProjectRoot: "\."' /Users/danleemh/ai/claude/evolve-loop/go/internal/swarm/dispatcher_test.go \
  && echo FAIL || echo PASS
```

Expected: `PASS`

---

## Criterion 4 — Tests still pass after the fix [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... ./internal/phases/swarmrunner/... -count=1 2>&1 | \
  grep -E "^(ok|FAIL)" | tee /dev/stderr | grep -v "^FAIL" | wc -l | \
  xargs -I{} test {} -ge 2 && echo PASS || echo FAIL
```

Expected: `PASS` (both packages pass)

---

## Criterion 5 — worktreeBase guard rejects a relative result [code]

```bash
# After the guard is added, the new test must exist and pass
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/swarm/... -run TestWorktreeBase -v 2>&1 | grep -E "PASS|FAIL" | head -5
```

Expected: all `TestWorktreeBase*` tests pass (including any new guard test).
