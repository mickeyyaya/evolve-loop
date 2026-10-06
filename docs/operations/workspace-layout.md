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
  behind. `evolve worktree cleanup --dev <task> --project-root runtime` removes
  the tree and deletes its branch only when the tree is clean and its head is
  in the fetched `origin/main` or `gh` reports a merged PR for the branch whose
  head commit is the tree's local head (git cannot see a squash merge; a merged
  PR at any other commit proves nothing about later or reused-name work); a dirty tree or an unmerged branch exits 1 and
  names the cause, a git failure exits 2, and both keep the tree and branch.
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
