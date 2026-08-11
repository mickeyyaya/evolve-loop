# Eval: core-worktree-relative-base-guard

## Objective

`gitWorktree.Create()` in `go/internal/core/worktree.go` must refuse a relative
worktree base (from EVOLVE_WORKTREE_BASE or a relative projectRoot) before
touching the filesystem — mirroring the guard added to `swarm/provision.go`
in cycle 294.

---

## Criterion 1 — Create() refuses relative EVOLVE_WORKTREE_BASE before MkdirAll [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/core/... -run TestGitWorktree_RelativeBaseRefused -v -count=1
```

**Expected:** `--- PASS: TestGitWorktree_RelativeBaseRefused`

---

## Criterion 2 — Create() refuses relative projectRoot (no EVOLVE_WORKTREE_BASE set) [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/core/... -run TestGitWorktree_RelativeProjectRootRefused -v -count=1
```

**Expected:** `--- PASS: TestGitWorktree_RelativeProjectRootRefused`

---

## Criterion 3 — Full core test suite stays green [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/core/... -count=1 -timeout=60s
```

**Expected:** `ok  github.com/mickeyyaya/evolve-loop/go/internal/core`

---

## Negative case — error message mentions "absolute" so the failure is actionable [code]

```bash
cd "$(git rev-parse --show-toplevel)/go"
go test ./internal/core/... -run TestGitWorktree_RelativeBaseRefused -v -count=1 2>&1 | grep -v "FAIL\|PASS\|RUN" || true
# test body verifies err.Error() contains "absolute"
go test ./internal/core/... -run TestGitWorktree_RelativeBaseRefused -v -count=1
```

**Expected:** `--- PASS: TestGitWorktree_RelativeBaseRefused`
