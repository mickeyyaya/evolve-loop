---
score_cap:
  - criterion: "deleteBranch runs git branch -d with LC_ALL=C so squash-landed branches are deleted under a non-English locale"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDeleteBranch_SquashLandedDeleteWithNonEnglishLocale$' ./internal/gc"
  - criterion: "an unmerged branch that is not squash-landed stays refused"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestDeleteBranch_UnmergedBranchStaysRefusedUnderLocaleOverride$' ./internal/gc"
---

# Eval: gc deleteBranch locale independence

> Pins that `deleteBranch` in `go/internal/gc/worktrees.go` matches git's stderr
> "not fully merged" only after forcing `LC_ALL=C`. Source: cycle 1861 inbox
> item gc-branch-delete-locale.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| locale-forced | squash-landed branch deleted under fr locale | 6/10 | `go test -run TestDeleteBranch_SquashLandedDeleteWithNonEnglishLocale ./internal/gc` |
| safety-kept | unmerged branch refused | 7/10 | `go test -run TestDeleteBranch_UnmergedBranchStaysRefusedUnderLocaleOverride ./internal/gc` |
