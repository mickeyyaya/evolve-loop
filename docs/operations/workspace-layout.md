# Workspace layout — the hub (since 2026-09-01)

One folder owns everything. A bare repository is the single git store, and every
checkout is a named worktree under it. This layout replaced the former sibling layout
(`evolve-loop` + `evolve-loop-runtime` + ad-hoc `evolve-loop-<task>` siblings).
The sibling layout was operationally correct, but it was hard for a human eye to read.

```
~/ai/claude/evolve-loop/
├─ .repo.git/          bare store (origin = github.com/mickeyyaya/evolve-loop);
│                      every worktree links here — one object store, no clones
├─ console/            interactive cockpit — detached HEAD, never holds main;
│                      Claude Code sessions start HERE
├─ runtime/            owns the `main` checkout; `evolve loop` runs here; live
│                      .evolve state (ledger, inbox, evals, instincts) lives here;
│                      cycle worktrees spawn under runtime/.evolve/worktrees/
├─ dev/                ephemeral task worktrees — created per task, deleted on
│                      merge (`evolve worktree create --dev <task> --branch <b>`)
├─ backups/            ref bundles + runs archives (`evolve backups verify [--dir D]`
│                      proves every head and patch exists elsewhere before you delete)
├─ go -> console/go            compat shim for pre-migration hook references
└─ .evolve -> console/.evolve  compat shim (safe to remove once no old sessions)
```

`evolve backups verify` is read-only. For each bundle head, it prints `IN_MAIN`, `OTHER_BRANCH` or `ONLY_IN_BACKUP`.
For each patch, it prints `APPLIED` or `UNAPPLIED`. `APPLIED` means that the reverse of the patch applies to the `origin/main` tree.
The command ignores the checkout and the index. The exit codes are:

- 0 = safe to delete;
- 1 = an unsafe head or patch (named on stderr);
- 2 = an I/O or git failure.

It never deletes.

## Why two long-lived planes (unchanged from the sibling era)

1. **Git allows a branch in only one worktree.** The loop ships to `main`, so
   the plane that runs the loop must hold `main`. The console stays detached.
2. **A live loop and an interactive session must not share a tree.** The
   tree-diff guard kills lanes when the tree becomes dirty unexpectedly. The loop
   also pins its own binary SHA: a rebuild mid-batch in the same tree breaks the
   anti-tamper check. So the planes have separate trees and separate binaries.

## Rules

- Dev work: always use a fresh worktree under `dev/`, branched from `origin/main`, and
  remove it after merge. `evolve worktree create --dev <task> --branch <b> --project-root runtime`
  fetches `origin/main` and adds `dev/<task>` on the new branch `<b>` (no upstream).
  It exits 1 when `dev/<task>` or branch `<b>` already exists. It exits 2 on a git
  failure (the fetch included). In both cases, it leaves nothing behind.
- Dev cleanup: `evolve worktree cleanup --dev <task> --project-root runtime`
  removes the tree and deletes its branch only after a proof. One of three proofs
  must show that all the work of the tree is already in the fetched `origin/main`.
  The command tries the proofs in this order:
  1. **Head in main.** The tree is clean and its head is in `origin/main`.
  2. **Content landed (2026-10-06).** Every change in the tree is already in
     `origin/main`. This is the proof for console work that reaches main by
     cherry-pick or as a patch in a boundary train. There, no merged PR has the
     head of the lane.

     The change is the commits of the branch since the merge-base, plus the staged
     and unstaged edits. That is, every path that `git diff --raw <merge-base>`
     lists. The proof judges each path on its own:
     - **Deleted in the tree.** It must be gone from main too.
     - **Present in the tree.** It must be in main, with the mode of the tree if
       the tree changed the mode. Then one of these conditions must be true:
       - the content is equal to the content in main;
       - a three-way `git merge-file` of the tree version into the main version is clean
         and leaves the content of main byte-identical. The merge uses the merge-base
         version as the common ancestor.
     - **Binary or symlink.** It counts only when it is equal to the version in main.
     - **Untracked.** Every untracked file that is not ignored must exist in
       `origin/main` with the same content.
  3. **Merged PR.** The tree is clean. `gh` reports a merged PR for the branch, with
     the local head of the tree as its head commit. git cannot see a squash merge.
     A merged PR at any other commit proves nothing about later work or work with a reused name.

  In all other cases, the command refuses with exit 1 and the reason, and it keeps the
  tree and the branch. For a dirty tree with unlanded work, the reason names the first
  file that is not in main. For a clean, unmerged branch, the reason names the failure
  of the content proof and the `gh` result. A git failure exits 2.

  The command removes a clean tree with a plain `git worktree remove`. It removes a
  dirty tree with `--force` only after a second read finds the tree exactly as it was
  proven. It deletes the branch with `update-ref -d` against the proven head.

  Ignored files are build output and runtime state (`go/bin`, `.evolve/*`, `.commit-gate/`).
  They are not work, and they go with the tree, the same as with the `worktree remove`
  command of git. `docs/private/` is not ignored, so a note there counts as untracked
  work that must be in main.

  The content proof is strict where it must be.
  - **Later edits elsewhere are tolerated.** Main can edit a path again after the
    landing, in a different part of the same file. The merge then still leaves
    main unchanged.
  - **Moved or deleted paths are refused.** A file that main has since moved
    or deleted counts as not in main. For example, a later landing can consume a
    filed inbox item into `.evolve/inbox/consumed/`. Then the item is no longer at its
    old path, so the command keeps such a tree and names the path.
  - **Edits that overlap are refused.** The merge conflicts when main changed the same
    lines again, or inserted lines next to the lines of the lane. Examples are the top
    of `CHANGELOG.md` and the size-ratchet allowances. The command keeps the tree and
    names the path. It never removes the tree.
  - **Why not `git apply --reverse --check`.** The first build used it. A
    review found that git apply searches for a hunk at shifted line offsets
    anywhere in the file. So an edit whose text already exists in another identical
    block of the main copy passed as landed, and a dirty tree was force-removed.
    `TestWorktreeDevCleanup_AnEditMatchingAnIdenticalBlockElsewhereInMainIsNotLanded`
    pins that case. The merge aligns lines through diffs against the merge-base, so
    it never searches for a match elsewhere in the file.
  - **The API.** The proof is in the leaf package `internal/landed`
    ([internal-landed.md](../architecture/packages/internal-landed.md)).
    `landed.Changes(ctx, worktree, base)` judges the tracked changes, and
    `landed.Untracked(ctx, worktree, files)` judges the untracked files. Each returns a
    `Verdict` (landed, or the reason that it is not). It judges a worktree only. A
    commit-tree variant (the full-tree commit of a checkpoint) comes back, together
    with its tests, when the checkpoint prune converges onto this proof.
  - **Tracked and untracked files are normalized differently.** The proof compares the
    working-tree bytes of a tracked path raw with the blob in main. It hashes an
    untracked file with `git hash-object`, which applies the clean filters, as
    `git add` does. Under `core.autocrlf` or Git LFS, the raw bytes of a tracked path
    differ from its stored blob, so the command keeps such a tree. That error is
    conservative: it can only keep a tree, never remove one.
- `--dry-run` prints what the command will remove and why it keeps each tree. It
  removes nothing, and it does not fetch. It judges against the current `origin/main`
  of the store and says which commit that is.
- `--dev --all` runs the cleanup over every directory under `dev/`. It removes
  each tree that a proof shows as landed. For every other tree, it prints
  `kept dev/<task>: <reason>`. It ends with `N removed, M kept, F failed`. It exits 0
  unless a task hit a git failure (exit 2).
  - **The quiet period.** `--all` also keeps any tree that changed within the
    quiet period. The time of the last change is the newest mtime in two sets of files.
    One set is the git admin files of the tree (`HEAD`, `index`, `logs/HEAD`). The other
    set is all changed or untracked files.
    A lane that was just created has no changes yet, so the proofs alone will remove it.

    The period is `gc.worktrees.dev_quiet_minutes` in the `.evolve/policy.json` of the
    plane, beside `min_age_minutes`. Unset, zero or negative means the default of 120
    minutes. The command raises any value below 30 minutes to 30, so the setting can
    never turn the grace off.
    `TestWorktreeDevCleanupAll_AFreshlyEditedFileKeepsATreeWhoseGitFilesAreOld`
    (the files of git are 3 hours old, and one file was edited moments ago) and
    `TestWorktreeDevCleanupAll_TheQuietPeriodComesFromThePlanesPolicy` pin this behaviour.

    The clock fails closed:
    - **Exact names.** The changed files come from `git status --porcelain -z`.
      So the command stats a name that git quotes (any non-ASCII name) by its real name.
    - **Unreadable paths.** If the command cannot stat a path for any reason other than
      its deletion, the tree fails (exit 2). The command does not treat the path as old.

    `…AFreshlyEditedFileWithANonASCIINameKeepsTheTree`,
    `…AFreshUnstagedEditToATrackedFileKeepsTheTree` and
    `TestLastChange_AnUnreadablePathIsAnErrorNotAnOldTree` pin this behaviour.
  - **No fetch per tree.** `--all` fetches once (not at all with `--dry-run`).
    Every git read in a tree runs with `GIT_OPTIONAL_LOCKS=0`. So the judgment of a
    tree never refreshes its index or moves its quiet-period clock.
  - **A directory that is not a worktree.** The command keeps and names a directory
    that is not a worktree of the hub store. It never touches that directory.
  - **Run at boundaries.** The console runs `--dev --all` at every wave
    boundary (runtime-reference.md, "Wave boundary"). The loop never removes a
    dev tree.
- Checkpoints (2026-10-06): the store holds the checkpoint refs of the operator,
  `refs/checkpoints/<worktree>/<UTC timestamp>` (`evolve checkpoint`). Each
  one is a commit pair: the staged tree on HEAD, then the full working tree on
  that commit.
  - **Never deleted by cleanup.** No cleanup deletes a checkpoint ref. No cleanup
    runs an expire or a prune that can drop the objects of a checkpoint.
    `cleanup --dev` deletes only `refs/heads/<branch>`, and the gc worktree
    sweep only runs `git branch -d` on `cycle-*` branches.
  - **Never a license to remove.** The command keeps a tree whose current content is
    not landed, even when its newest checkpoint holds that content exactly. The landed
    proofs above are the only removal proofs. So a sweep never removes unsaved work
    or work that is only checkpointed.
  - **Pins.** `TestWorktreeDevCleanup_RemovingALandedTreeKeepsEveryCheckpointRefAndItsCommits`,
    `TestWorktreeDevCleanup_AnUnlandedTreeIsKeptEvenWhenItsNewestCheckpointHoldsIt`,
    and the checkpoint refs in the gc enforce-safety test.
  - **Open follow-up.** The checkpoint verb and `internal/landed` are
    both on main. The `origin/main` ref has one spelling,
    `plane.OriginMainRef`. `evolve checkpoint prune --landed` still uses its
    own, stricter, equality-only `wtcheckpoint.hasLanded`. Two steps remain:
    1. **A commit-tree mode.** Before the prune can call `landed.Changes`, it
       needs this mode, because a checkpoint is a commit, not a worktree.
    2. **A looser prune.** To move the prune from exact equality to the
       merge proof is a separate, reviewed component.

    The inbox item `checkpoint-prune-converges-on-landed-proof` records this follow-up.
- Plane sync: merge-only (`git merge origin/main`). Never rebase a plane.
- Fresh worktree: run `make -C go build` before any `evolve` command.
- Bare-store notes: the store carries the standard refspec
  (`+refs/heads/*:refs/remotes/origin/*`). If you move the hub directory,
  run `git --git-dir=.repo.git worktree repair <worktree paths>` to fix the
  bidirectional links.

## Checkpoints (`refs/checkpoints/`)

Console lanes keep their work as uncommitted changes in `dev/` worktrees until it lands.
These events can lose that work: an agent that dies, a removed worktree, a reset, a checkout
or a churn-discard. `evolve checkpoint` keeps a copy in the bare store, where every worktree
can see it ([ADR-0122](../architecture/adr/0122-checkpoint-uncommitted-worktree-work-as-refs.md)).

- **Where it works.** In the hub only: the project root (`--project-root`, default `.`) must be
  a worktree root of the bare store. `plane.ResolveHub` resolves the hub, its `dev/` directory and
  `refs/remotes/origin/main`. This is the same rule that `evolve worktree create|cleanup --dev` use.
- **Namespace.** `refs/checkpoints/<worktree>/<UTC yyyymmddThhmmssZ>`. `<worktree>` is the name of
  the worktree directory (`dev/cl-gc` is `cl-gc`).
  - Each ref names a commit whose tree is the full working tree. The full tree includes untracked
    files. It excludes ignored files, `go/evolve` and `.evolve/ledger.*`.
  - The tree of its parent is what was staged. Its grandparent is the `HEAD` that the work was based on.
  - They are ordinary refs in `.repo.git`: no branch, no reflog, and by default no fetch or push.
    They keep the base commit reachable after its branch is deleted.
  - Only the console and `evolve checkpoint` write them.
- **Save.** Run `evolve checkpoint save` in a dev worktree. Or run `evolve checkpoint save --all`
  from any directory in the hub to cover every dev worktree (never the runtime plane or a cycle worktree).
  Nothing in the worktree changes. Each worktree keeps its newest `checkpoint.keep_per_worktree`
  checkpoints (default 20).
- **Find.** Run `evolve checkpoint list [--worktree <name>]`, or `git --git-dir=.repo.git for-each-ref refs/checkpoints`.
- **Restore.** `evolve checkpoint restore <name>/<stamp> --into dev/<new-task>` creates a new worktree
  at the saved base. The new worktree has the staged, unstaged and untracked files exactly as saved.
  Then run `git switch -c <branch>` in it to continue on a branch. A restore into the original worktree
  works when that worktree is clean (for example, after a `reset --hard`). The command refuses a dirty target.
- **Clean up.** `evolve checkpoint prune` applies the retention. `--landed` also drops the checkpoints
  whose changes are already on `origin/main`. When `evolve worktree cleanup --dev` removes a dev worktree,
  the checkpoints of that worktree stay in place.
- **By hand.** This is the same snapshot with raw git, as the console first took it:
  1. Copy the index file of the worktree. Run `GIT_INDEX_FILE=<copy> git write-tree` for the staged tree.
  2. Copy the index file again. Run `GIT_INDEX_FILE=<copy2> git add -A -- . ':!go/evolve' ':!.evolve/ledger.*'`
     and `write-tree` for the full tree.
  3. Run `commit-tree` for the staged tree on `HEAD`, then for the full tree on that commit.
     Run `update-ref` for the result.

  To restore by hand into a clean worktree at the base, run `git read-tree -m -u HEAD <ref>^{tree}`,
  then `git read-tree <ref>^^{tree}`.
