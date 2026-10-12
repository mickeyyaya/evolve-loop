---
score_cap:
  - criterion: "a failed git worktree add in runWorktreeCreateDev leaves no branch behind and a retry succeeds"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestWorktreeDev_FailedCreateLeavesNoBranch$' ./cmd/evolve"
  - criterion: "the uncreatable dev dir refusal case asserts branchOK false"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRunWorktreeCreateDev_RefusalsAndGitFailures$' ./cmd/evolve"
---

# Eval: evolve worktree create --dev cleans up the branch when worktree add fails

> Pins that `runWorktreeCreateDev` deletes the branch created by a failed `git worktree add -b`, so the next create does not refuse with "branch already exists". Source: inbox item worktree-create-dev-leaves-branch (cycle 1859).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| leftover-branch | failed create leaves no branch, retry returns 0 | 4/10 | `go test -run '^TestWorktreeDev_FailedCreateLeavesNoBranch$' ./cmd/evolve` |
| table-case | uncreatable dev dir case requires no branch | 5/10 | `go test -run '^TestRunWorktreeCreateDev_RefusalsAndGitFailures$' ./cmd/evolve` |
