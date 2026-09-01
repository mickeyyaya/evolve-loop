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
│                      merge (`git worktree add dev/<task> -b <branch> origin/main`)
├─ backups/            ref bundles + runs archives
├─ go -> console/go            compat shim for pre-migration hook references
└─ .evolve -> console/.evolve  compat shim (safe to remove once no old sessions)
```

## Why two long-lived planes (unchanged from the sibling era)

1. **Git allows a branch in only one worktree.** The loop ships to `main`, so
   the plane running the loop must hold `main`; the console stays detached.
2. **A live loop and an interactive session must not share a tree** — the
   tree-diff guard kills lanes when the tree dirties unexpectedly, and the loop
   pins its own binary SHA (rebuilding mid-batch in the same tree breaks the
   anti-tamper check). Separate trees, separate binaries.

## Rules

- Dev work: always a fresh worktree under `dev/`, branched from `origin/main`,
  removed after merge (`git worktree remove`, `git branch -D`).
- Plane sync: merge-only (`git merge origin/main`) — never rebase a plane.
- Fresh worktree: `make -C go build` before any `evolve` command.
- Bare-store notes: the store carries the standard refspec
  (`+refs/heads/*:refs/remotes/origin/*`); if the hub directory is ever moved,
  run `git --git-dir=.repo.git worktree repair <worktree paths>` to fix the
  bidirectional links.
