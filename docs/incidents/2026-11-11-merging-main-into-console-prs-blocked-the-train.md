# 2026-11-11: merging main into console PRs blocked the integration train

Related documents: the train code `go/internal/integration/train.go` and the incident record [2026-11-09-duplicate-ratchet-misses-staged-predicates.md](2026-11-09-duplicate-ratchet-misses-staged-predicates.md) (same wave range).

## Summary

- **Impact:** about one hour of console time was lost at the wave 165 boundary. Two PRs, #1390 and #1397, were affected.
- **Root cause:** the train composes each recorded PR as a linear patch series onto the held base. A merge commit has no single linear diff to replay, so the train refuses it by design.
- **Trigger:** the console merged main into the two PRs to clear conflicts. `evolve integration record-pr` then refused #1390 because its head was a merge commit.
- **Fix:** a procedure. Never merge main into a console PR. Rebuild a conflicting PR as a new branch from `origin/main`.
- **Note:** `evolve pr refresh` does not help. It regenerates derived files only. It does not update a PR branch.

## Terms used throughout

- **The train:** the integration train. It composes the recorded PRs onto a held base (`go/internal/integration/train.go`).
- **A seat:** one reviewer run through `evolve agent run`. Its review record binds to the staged tree of the worktree it ran against.
- **A scoped gate:** the commit gate run with `--files` on the touched files only.
- **A linear PR:** a PR whose commits are a plain series of patches on one base. It has no merge commit.

## Timeline

The fact sheet gives the date and the wave boundary only. It gives no clock times.

| Time | Event | Evidence |
|---|---|---|
| 2026-11-11, wave 165 boundary | The console merges main into PRs #1390 and #1397 to clear conflicts. | the fact sheet of this record |
| same period | Each merge needs seats bound to its worktree, a scoped gate (`--files`) and a fix of the lint debt of main in the touched packages. | the fact sheet of this record |
| same period | `evolve integration record-pr` refuses #1390. Its head is a merge commit. | the refusal text at `go/internal/integration/train.go:473` |

## Root cause

**F1.1** The train reads the commits of a PR and refuses a commit with more than one parent. The refusal reads: "commit <sha> is a merge of <parent> and <parent>, and the train composes a linear PR. Next: put the commits on a new branch from origin/main, open a PR, and run record-pr on it." (`mergeReason`, `go/internal/integration/train.go:468-474`).
*Implication:* the refusal is a design rule, not a defect.

**F1.2** The train replays each commit by its patch identity (`CommitIdentity` with `PatchID`, `train.go:452-455`). A merge commit has two parents and no single patch.
*Implication:* the train has no diff to replay for a merge commit.

**F1.3** `evolve pr refresh` commits only the derived files `signal-codes`, `darwin-set` and `research-index` (`go/cmd/evolve/cmd_pr_refresh.go:20`). It stops when a merge, rebase or cherry-pick is in progress (`prRefreshOpHeads`, line 21).
*Implication:* it is not a way to bring a PR branch up to date with main.

**F1.4** The chain: a PR conflicts with main. The console merges main into the PR branch. The head of the branch is now a merge commit. `record-pr` refuses the head.
*Implication:* the PR must be rebuilt anyway, so the work of the merge is lost.

**F1.5** Each merge needed seats bound to its worktree, a scoped gate and a fix of the lint debt of main in the touched packages.
*Implication:* the merge cost time before the refusal, and the refusal made that time a loss.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Never merge main into a console PR. | F1.1, F1.2 | Merge main and then squash the merge: the console then needs a new review of the squashed tree, with no gain over a rebuild. |
| D2 | Rebuild a conflicting PR as a new branch from `origin/main`: `git restore --source=<old head or a resolved tree> --staged --worktree -- .`. | F1.1, F1.4 | Use `evolve pr refresh` to update the branch: it regenerates derived files only (F1.3). |
| D3 | Run the seats against the worktree of the new branch, review the whole diff, gate it, and record the new PR. | F1.4 | Reuse the old review records: a seat record binds to the staged tree, so an old record does not match. |
| D4 | Give a multi-commit PR one review of its whole change. `record-pr --reviewed` reads the review record at the digest of the PR diff. | F1.2 | Review each commit alone: the record is read at the digest of the whole PR diff. |

The claims in D3 and D4 about the binding of a seat record and the digest of the PR diff come from the fact sheet. This record did not re-check them in the code.

## What changed

| Change | Regression test |
|---|---|
| The refusal of a merge commit with a typed next step already exists on main (`go/internal/integration/train.go:468-474`). | `TestTrainRecordRefusesAMergeCommitWithATypedNextStep` in `go/internal/integration/consolepr_record_test.go:153` |
| The procedure D1 to D4 is operator practice. This record adds no code. | none |

The fact sheet names no pull request for the fix, so none is cited. The PR numbers #1390 and #1397 come from the fact sheet. This record did not check them against GitHub.

## What we learned

- A gate that composes by patch needs a linear history. A rule of the gate is a rule of the operator: do not merge main into a console PR.
- A conflict is a reason to rebuild from `origin/main`, not to merge. The rebuild keeps the review record valid, because the seats run against the new worktree.
- The guard is the refusal text. It names the next step: put the commits on a new branch from `origin/main`. Follow it at the first conflict.
