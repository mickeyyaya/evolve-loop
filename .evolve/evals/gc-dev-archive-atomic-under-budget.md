---
score_cap:
  - criterion: "A checkpoint save that the sweep budget cuts still finishes its archive: tree removed, one checkpoint, one manifest row"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestApplyDevWorktrees_ArchiveRunsUnderDedicatedContext$' ./internal/gc"
  - criterion: "A worktree remove that starts after the budget ends still finishes and is recorded"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestApplyDevWorktrees_ABudgetThatEndsAsTheRemoveStartsStillFinishesTheArchive$' ./internal/gc"
  - criterion: "A budget cut mid-sweep leaves every tree untouched or fully archived, and defers the tree not yet started"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestApplyDevWorktrees_BudgetCutLeavesTreeUntouchedOrFullyArchived$' ./internal/gc"
  - criterion: "Every internal/gc test still passes, the earlier budget deferral tests included"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/gc"
---

# Eval: The dev-worktree archive is atomic under the sweep budget

> This eval pins one rule of `ApplyDevWorktrees` in `go/internal/gc/devworktrees_apply.go`. An archive that has started runs to its end: checkpoint ref, worktree remove and manifest row. The sweep budget can stop a tree before its archive starts, but not during it. Source incident: cycle 1862, inbox item `gc-dev-archive-atomic-under-budget`. The bug-reproduction phase showed a cut after the checkpoint save left `exists=true, hasRef=true, hasManifest=false`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cut-after-save | The save cut finishes the archive | 8/10 | `go test -run TestApplyDevWorktrees_ArchiveRunsUnderDedicatedContext` |
| cut-at-remove | The remove cut finishes the archive | 8/10 | `go test -run TestApplyDevWorktrees_ABudgetThatEndsAsTheRemoveStartsStillFinishesTheArchive` |
| atomic-or-untouched | No half-archived tree; the next tree is deferred | 9/10 | `go test -run TestApplyDevWorktrees_BudgetCutLeavesTreeUntouchedOrFullyArchived` |
| preservation | The gc package stays green | 7/10 | `go test ./internal/gc` |
