# Workspace layout — the hub (since 2026-09-01)

One folder owns everything; a bare repository is the single git store and every
checkout is a named worktree under it. This replaced the former sibling layout
(`evolve-loop` + `evolve-loop-runtime` + ad-hoc `evolve-loop-<task>` siblings),
which was operationally correct but illegible to a human eye.

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

`evolve backups verify` is read-only: each bundle head prints `IN_MAIN`, `OTHER_BRANCH` or `ONLY_IN_BACKUP`, each patch `APPLIED` (its reverse applies to the `origin/main` tree; the checkout and index are ignored) or `UNAPPLIED`. Exit 0 = safe to delete, 1 = an unsafe head or patch (named on stderr), 2 = I/O or git failure. It never deletes.

## Why two long-lived planes (unchanged from the sibling era)

1. **Git allows a branch in only one worktree.** The loop ships to `main`, so
   the plane running the loop must hold `main`; the console stays detached.
2. **A live loop and an interactive session must not share a tree** — the
   tree-diff guard kills lanes when the tree dirties unexpectedly, and the loop
   pins its own binary SHA (rebuilding mid-batch in the same tree breaks the
   anti-tamper check). Separate trees, separate binaries.

## Rules

- Dev work: always a fresh worktree under `dev/`, branched from `origin/main`,
  removed after merge. `evolve worktree create --dev <task> --branch <b>
  --project-root runtime` fetches `origin/main` and adds `dev/<task>` on the new
  branch `<b>` (no upstream); it exits 1 when `dev/<task>` or branch `<b>`
  already exists and 2 on a git failure (the fetch included), leaving nothing
  behind.
- Dev cleanup: `evolve worktree cleanup --dev <task> --project-root runtime`
  removes the tree and deletes its branch only when one of three proofs shows
  that all of the tree's work is already in the fetched `origin/main`. The
  proofs are tried in this order:
  1. **Head in main.** The tree is clean and its head is in `origin/main`.
  2. **Content landed (2026-10-06).** Every change in the tree is already in
     `origin/main`. This is the proof for console work that reaches main by
     cherry-pick or as a patch in a boundary train, where no merged PR has the
     lane's head. The change is the branch's commits since the merge-base plus
     the staged and unstaged edits: every path `git diff --raw <merge-base>`
     lists. Each path is judged on its own:
     - **Deleted in the tree.** It must be gone from main too.
     - **Present in the tree.** It must be in main, with the tree's mode if
       the tree changed the mode. Then either its content must equal main's,
       or a three-way `git merge-file` of the tree's version into main's (the
       merge-base version as the common ancestor) must be clean and leave
       main's content byte-identical.
     - **Binary or symlink.** It counts only when it is equal to main's.
     - **Untracked.** Every untracked file that is not ignored must exist in
       `origin/main` with the same content.
  3. **Merged PR.** The tree is clean and `gh` reports a merged PR for the
     branch whose head commit is the tree's local head. git cannot see a
     squash merge, and a merged PR at any other commit proves nothing about
     later or reused-name work.

  Anything else is refused with exit 1 and the reason, and the tree and branch
  are kept. A dirty tree with unlanded work names the first file that is not in
  main, and a clean, unmerged branch names the content proof's failure and the
  `gh` result. A git failure exits 2. A clean tree is removed with a plain
  `git worktree remove`. A dirty tree is removed with `--force` only after a
  second read finds it exactly as it was proven. The branch is deleted with
  `update-ref -d` against the proven head. Ignored files are build output and
  runtime state (`go/bin`, `.evolve/*`, `.commit-gate/`). They are not work,
  and they go with the tree, as they do under git's own `worktree remove`.
  `docs/private/` is not ignored, so a note there counts as untracked work that
  must be in main.

  The content proof is strict where it must be.
  - **Later edits elsewhere are tolerated.** Main may have edited a path again
    after the landing, elsewhere in the same file, and the merge still leaves
    main unchanged.
  - **Moved or deleted paths are refused.** A file that main has since moved
    or deleted counts as not in main. For example, a filed inbox item that a
    later landing consumed into `.evolve/inbox/consumed/` is no longer at its
    old path, so such a tree is kept and the path named.
  - **Overlapping edits are refused.** When main changed the same lines again,
    or inserted next to the lane's lines (the top of `CHANGELOG.md`, the
    size-ratchet allowances), the merge conflicts. The tree is kept and the
    path named, never removed.
  - **Why not `git apply --reverse --check`.** The first build used it. A
    review found that git apply searches for a hunk at shifted line offsets
    anywhere in the file, so an edit whose text already exists in another,
    identical block of main's copy passed as landed, and a dirty tree was
    force-removed. That case is pinned by
    `TestWorktreeDevCleanup_AnEditMatchingAnIdenticalBlockElsewhereInMainIsNotLanded`.
    The merge aligns lines through diffs against the merge-base, so it never
    searches for a match elsewhere in the file.
  - **The API.** The proof lives in the leaf package `internal/landed`
    ([internal-landed.md](../architecture/packages/internal-landed.md)).
    `landed.Changes(ctx, worktree, base)` judges the tracked changes and
    `landed.Untracked(ctx, worktree, files)` the untracked files; each returns a
    `Verdict` (landed, or the reason it is not). It judges a worktree only. A
    commit-tree variant (a checkpoint's full-tree commit) comes back together
    with its tests when the checkpoint prune converges onto this proof.
  - **Tracked and untracked files are normalized differently.** A tracked
    path's working-tree bytes are compared raw with main's blob. An untracked
    file is hashed with `git hash-object`, which applies the clean filters, as
    `git add` would. Under `core.autocrlf` or Git LFS, a tracked path's raw
    bytes differ from its stored blob, so such a tree is kept. That errs
    conservative: it can only keep a tree, never remove one.
- `--dry-run` prints what would be removed and why each tree is kept. It
  removes nothing, and it does not fetch: it judges against the store's current
  `origin/main` and says which commit that is.
- `--dev --all` runs the cleanup over every directory under `dev/`. It removes
  each tree a proof shows landed, prints `kept dev/<task>: <reason>` for every
  other, and ends with `N removed, M kept, F failed`. It exits 0 unless a task
  hit a git failure (exit 2).
  - **The quiet period.** `--all` also keeps any tree changed within the
    quiet period: the newest mtime of its git admin files (`HEAD`, `index`,
    `logs/HEAD`) or of any changed or untracked file. A lane that has just been
    created has no changes yet, so the proofs alone would remove it. The period
    is `gc.worktrees.dev_quiet_minutes` in the plane's `.evolve/policy.json`,
    beside `min_age_minutes`. Unset, zero or negative means the default of 120
    minutes, and any value below 30 minutes is raised to 30, so the setting can
    never turn the grace off. Pinned by
    `TestWorktreeDevCleanupAll_AFreshlyEditedFileKeepsATreeWhoseGitFilesAreOld`
    (git's files 3 hours old, one file edited moments ago) and
    `TestWorktreeDevCleanupAll_TheQuietPeriodComesFromThePlanesPolicy`.
    The clock fails closed:
    - **Exact names.** The changed files come from `git status --porcelain
      -z`, so a name git would quote (any non-ASCII name) is stat'ed by its
      real name.
    - **Unreadable paths.** A path it cannot stat for any reason other than
      its deletion fails the tree (exit 2) rather than looking old.

    Pinned by `…AFreshlyEditedFileWithANonASCIINameKeepsTheTree`,
    `…AFreshUnstagedEditToATrackedFileKeepsTheTree` and
    `TestLastChange_AnUnreadablePathIsAnErrorNotAnOldTree`.
  - **No fetch per tree.** `--all` fetches once (not at all with `--dry-run`).
    Every git read in a tree runs with `GIT_OPTIONAL_LOCKS=0`, so judging a
    tree never refreshes its index or moves its quiet-period clock.
  - **A directory that is not a worktree.** A directory that is not a worktree
    of the hub store is kept and named, never touched.
  - **Run at boundaries.** The console runs `--dev --all` at every wave
    boundary (runtime-reference.md, "Wave boundary"). The loop never removes a
    dev tree.
- Checkpoints (2026-10-06): the store holds the operator's checkpoint refs
  `refs/checkpoints/<worktree>/<UTC timestamp>` (`evolve checkpoint`). Each
  one is a commit pair: the staged tree on HEAD, then the full working tree on
  that commit.
  - **Never deleted by cleanup.** No cleanup deletes a checkpoint ref, and none
    runs an expire or prune that could drop a checkpoint's objects:
    `cleanup --dev` deletes only `refs/heads/<branch>`, and the gc worktree
    sweep only `git branch -d`s `cycle-*` branches.
  - **Never a license to remove.** A tree whose current content is not landed
    is kept even when its newest checkpoint holds it exactly. The landed
    proofs above are the only removal proofs, so unsaved or merely
    checkpointed work is never removed by a sweep.
  - **Pins.** `TestWorktreeDevCleanup_RemovingALandedTreeKeepsEveryCheckpointRefAndItsCommits`,
    `TestWorktreeDevCleanup_AnUnlandedTreeIsKeptEvenWhenItsNewestCheckpointHoldsIt`,
    and the checkpoint refs in the gc enforce-safety test.
  - **Remaining follow-up.** The checkpoint verb and `internal/landed` are
    both on main, and the `origin/main` ref has one spelling,
    `plane.OriginMainRef`. `evolve checkpoint prune --landed` still uses its
    own, stricter, equality-only `wtcheckpoint.hasLanded`. Two steps remain:
    1. **A commit-tree mode.** Calling `landed.Changes` from the prune first
       needs this mode, because a checkpoint is a commit, not a worktree.
    2. **Loosening the prune.** Moving the prune from exact equality to the
       merge proof is a separate, reviewed component.

    Filed as the inbox item `checkpoint-prune-converges-on-landed-proof`.
- Plane sync: merge-only (`git merge origin/main`) — never rebase a plane.
- Fresh worktree: `make -C go build` before any `evolve` command.
- Bare-store notes: the store carries the standard refspec
  (`+refs/heads/*:refs/remotes/origin/*`); if the hub directory is ever moved,
  run `git --git-dir=.repo.git worktree repair <worktree paths>` to fix the
  bidirectional links.

## Checkpoints (`refs/checkpoints/`)

Console lanes keep their work as uncommitted changes in `dev/` worktrees until it lands, so an agent dying, a removed worktree, a reset, a checkout or a churn-discard could lose it. `evolve checkpoint` keeps a copy in the bare store, where every worktree can see it ([ADR-0122](../architecture/adr/0122-checkpoint-uncommitted-worktree-work-as-refs.md)).

- **Where it works.** In the hub only: the project root (`--project-root`, default `.`) must be a worktree root of the bare store. The hub, its `dev/` directory and `refs/remotes/origin/main` are resolved by `plane.ResolveHub`, the same rule `evolve worktree create|cleanup --dev` use.
- **Namespace.** `refs/checkpoints/<worktree>/<UTC yyyymmddThhmmssZ>`, where `<worktree>` is the worktree directory's name (`dev/cl-gc` is `cl-gc`). Each ref names a commit whose tree is the full working tree (untracked files included, ignored files, `go/evolve` and `.evolve/ledger.*` excluded); its parent's tree is what was staged, and its grandparent is the `HEAD` the work was based on. They are ordinary refs in `.repo.git`: no branch, no reflog, not fetched or pushed by default, and they keep the base commit reachable after its branch is deleted. Only the console and `evolve checkpoint` write them.
- **Save.** `evolve checkpoint save` in a dev worktree, or `evolve checkpoint save --all` from anywhere in the hub to cover every dev worktree (never the runtime plane or a cycle worktree). Nothing in the worktree changes. Each worktree keeps its newest `checkpoint.keep_per_worktree` (default 20).
- **Find.** `evolve checkpoint list [--worktree <name>]`, or `git --git-dir=.repo.git for-each-ref refs/checkpoints`.
- **Restore.** `evolve checkpoint restore <name>/<stamp> --into dev/<new-task>` creates a new worktree at the saved base with the staged, unstaged and untracked files exactly as saved; then `git switch -c <branch>` inside it to continue on a branch. Restoring into the original worktree works when it is clean (after a `reset --hard`, say); a dirty target is refused.
- **Clean up.** `evolve checkpoint prune` applies the retention; `--landed` also drops checkpoints whose changes are already on `origin/main`. Removing a dev worktree with `evolve worktree cleanup --dev` leaves its checkpoints in place.
- **By hand.** The same snapshot with raw git, as the console first took it: copy the worktree's index file, run `GIT_INDEX_FILE=<copy> git write-tree` for the staged tree; copy it again, run `GIT_INDEX_FILE=<copy2> git add -A -- . ':!go/evolve' ':!.evolve/ledger.*'` and `write-tree` for the full tree; `commit-tree` the staged tree on `HEAD`, the full tree on that commit, and `update-ref` the result. To restore by hand into a clean worktree at the base: `git read-tree -m -u HEAD <ref>^{tree}`, then `git read-tree <ref>^^{tree}`.
