---
score_cap:
  - criterion: "`evolve worktree create --dev t --branch b` makes <hub>/dev/t as a worktree of the hub store on branch b at origin/main after a fetch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_006_' ./acs/cycle1799"
  - criterion: "`evolve worktree create --dev` refuses an existing task or an existing branch with exit 1 and leaves nothing behind"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_007_' ./acs/cycle1799"
  - criterion: "`evolve worktree create --dev` exits 2 when the git fetch of origin fails, leaving no tree or branch"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_008_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --dev t` refuses a dirty tree with exit 1, names the cause, and keeps the tree, the edit and the branch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_009_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --dev t` refuses an unmerged branch with exit 1, names the cause, and keeps the tree and branch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_010_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --dev t` removes a clean tree whose head is in origin/main together with its branch"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_011_' ./acs/cycle1799"
  - criterion: "A squash-merged PR counts as merged when gh reports it MERGED, and without gh the same branch is refused"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_012_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --dev t` refuses with exit 1, keeping the tree and the branch, when the local head carries a commit made after the merged PR's head (unpushed or pushed); a merged PR proves only the head gh reports as its headRefOid"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_013_' ./acs/cycle1799"
  - criterion: "`evolve worktree cleanup --dev t` refuses with exit 1 when the only merged PR for a reused branch name has a different head, and accepts once a merged PR's head is the local head"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1799_014_' ./acs/cycle1799"
---

# Eval: Console dev worktrees get `evolve worktree create --dev` and `evolve worktree cleanup --dev`

> Pins the `cli-worktree-dev` inbox item (console, 2026-09-30; built in cycle 1797). The worktree
> verbs were per-cycle only, so console work followed workspace-layout.md by hand: `git worktree add
> dev/<task> -b <branch> origin/main`, `make -C go build`, and after the merge `git worktree remove`
> plus `git branch -D`, first checking by hand that the PR merged, because git does not count a
> squash-merged branch as merged. The predicates drive the real `evolve` binary against a hub fixture
> (`.repo.git` bare store, `runtime/` worktree, a local origin that moves on after the hub is cloned)
> with a PATH that holds only `git` and, where needed, a fake `gh`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| create-at-fetched-origin | dev/t on branch b at the fetched origin/main | 7/10 | `go test -tags acs -run TestC1799_006_ ./acs/cycle1799` |
| create-collision (negative) | existing task or branch, exit 1 | 6/10 | `go test -tags acs -run TestC1799_007_ ./acs/cycle1799` |
| create-git-io (edge) | fetch failure, exit 2 | 5/10 | `go test -tags acs -run TestC1799_008_ ./acs/cycle1799` |
| cleanup-dirty (negative) | dirty tree, exit 1, cause named | 7/10 | `go test -tags acs -run TestC1799_009_ ./acs/cycle1799` |
| cleanup-unmerged (negative) | unmerged branch, exit 1, cause named | 7/10 | `go test -tags acs -run TestC1799_010_ ./acs/cycle1799` |
| cleanup-merged | tree and branch removed | 8/10 | `go test -tags acs -run TestC1799_011_ ./acs/cycle1799` |
| squash-merged-via-gh | gh MERGED counts; no gh refuses | 7/10 | `go test -tags acs -run TestC1799_012_ ./acs/cycle1799` |
| post-merge-commit (negative, audit H1) | a commit after the merged PR's head is refused, exit 1, and stays in a branch | 8/10 | `go test -tags acs -run TestC1799_013_ ./acs/cycle1799` |
| reused-branch-name (negative + control, audit H1) | an earlier PR under the same name proves nothing; the head's own merged PR does | 8/10 | `go test -tags acs -run TestC1799_014_ ./acs/cycle1799` |

## Model Grader
- `[model]` Rubric: "docs/operations/workspace-layout.md and docs/operations/runtime-reference.md describe `evolve worktree create --dev <task> --branch <b>` and `evolve worktree cleanup --dev <task>` (refusal causes and exit codes 1 and 2) in place of the raw `git worktree add` / `git worktree remove` / `git branch -D` steps." — threshold: >= 80

## Adversarial Cases
- Negative: the hub's local main is stale; only a fetch reaches the origin tip that create must branch from.
- Negative: an existing branch that no worktree holds must still be refused with exit 1, not surfaced as a git failure.
- Edge: a vanished origin makes the fetch fail; create exits 2 and leaves nothing behind.
- Cheapest gaming fake: removing whenever gh is absent fails the no-gh control in `TestC1799_012_`; after the squash, main edits the same file again, so a content comparison cannot prove the merge without gh.
- Data loss (cycle 1797 audit H1): the first build took any merged PR named by the head branch as proof (`gh pr list … --jq length`) and ran `git branch -D`, deleting a commit made after the merge and work on a reused branch name. The fake gh reports each merged PR's `headRefOid`; `TestC1799_013_` and `TestC1799_014_` require that a merged PR proves only its own head, and that the refused commit is still held by the branch.
- Ordering: the fake lists merged PRs newest first, as gh does. `TestC1799_014_`'s control lists the head's own PR ahead of the earlier one, so both an any-of check and a `.[0]` check pass. An implementation that demands exactly one merged PR fails.
