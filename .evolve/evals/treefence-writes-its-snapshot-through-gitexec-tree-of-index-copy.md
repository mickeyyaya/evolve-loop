---
score_cap:
  - criterion: "The four ambient git variable names are listed only in internal/gitexec, treefence imports gitexec, and Take ignores ambient GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE/GIT_COMMON_DIR"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1884_001 ./acs/cycle1884"
  - criterion: "treefence writes its snapshot trees through gitexec.TreeOfIndexCopy and Take yields the same tree as TreeOfIndexCopy without touching the real index"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1884_002 ./acs/cycle1884"
  - criterion: "Take on a worktree without a readable index is refused and Begin surfaces the refusal through TakeErr"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1884_003 ./acs/cycle1884"
  - criterion: "No index-copy routine calling os.Chtimes remains outside internal/gitexec"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1884_005 ./acs/cycle1884"
  - criterion: "All existing treefence and gitexec tests pass"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/treefence ./internal/gitexec"
---

# Eval: treefence writes its snapshot through gitexec.TreeOfIndexCopy

> Pins the refactor that removes treefence's private scratch-index tree writer
> (writeTreeMode, copyIndex) and its private ambientGitVars list, so the
> read-only phase fence (ADR-0097) and evolve checkpoint share one index-copy
> routine and one ambient-variable list in internal/gitexec. The allFiles
> missing-index behaviour is decided as a refusal, like TreeOfIndexCopy, and is
> pinned by a named predicate. Source: console lane cl-checkpoint architecture
> re-review, 2026-10-06; materialized in cycle 1884.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single-ambient-list | ambient names listed only in gitexec; Take ignores them | 7/10 | `go test -tags acs -run TestC1884_001 ./acs/cycle1884` |
| shared-tree-writer | Take goes through TreeOfIndexCopy, same tree, real index untouched | 8/10 | `go test -tags acs -run TestC1884_002 ./acs/cycle1884` |
| missing-index-refusal | Take without an index is refused; Begin reports TakeErr | 7/10 | `go test -tags acs -run TestC1884_003 ./acs/cycle1884` |
| one-index-copy | no os.Chtimes index copy outside gitexec | 6/10 | `go test -tags acs -run TestC1884_005 ./acs/cycle1884` |
| regression | treefence and gitexec suites pass | 8/10 | `go test -count=1 ./internal/treefence ./internal/gitexec` |
