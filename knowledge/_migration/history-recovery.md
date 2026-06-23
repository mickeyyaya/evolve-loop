# History recovery — the Stage-0 safety net

Before the consolidation deleted any cycle history or branches, everything was captured into a
recovery archive **outside the repo** so no cleanup can touch it.

## Where the safety net lives

```
~/ai/claude/evolve-loop-SAFETY-2026-05-30/
  RECOVERY.md                       # the canonical recovery instructions
  unmerged-refs.bundle              # 80M git bundle — 265 refs (all branches, remotes, tags, stashes)
  stash-patches/                    # 00-stash-list.txt + stash-0..4.patch
  worktree-uncommitted/             # egps-skip-fix.patch (14 files) + cycle-144.patch
```

Plus an in-repo restore tag: **`pre-consolidation-2026-05-30`** → commit `5436919` (main at start).

## Full revert

```
git reset --hard pre-consolidation-2026-05-30
```

## Recover one branch from the bundle

```
SAFETY=~/ai/claude/evolve-loop-SAFETY-2026-05-30
git bundle list-heads $SAFETY/unmerged-refs.bundle            # list all 265 captured refs
git fetch $SAFETY/unmerged-refs.bundle refs/heads/<branch>:<local-name>
# e.g. ship-recovery work:
git fetch $SAFETY/unmerged-refs.bundle refs/remotes/origin/feat/ws-g-multi-cli:recovered-ws-g
```

## Recover a stash

Stashes were converted to `stash-archive/N` branches inside the bundle (N=0..4, mapping in
[unmerged-work-ledger.md](unmerged-work-ledger.md)):

```
git fetch $SAFETY/unmerged-refs.bundle refs/heads/stash-archive/1:recovered-stash-1
```

## Re-apply an uncommitted worktree patch

```
git apply ~/ai/claude/evolve-loop-SAFETY-2026-05-30/worktree-uncommitted/egps-skip-fix.patch
```

## Provenance

The raw `.evolve/runs/` (172 cycles), `ledger.jsonl` (2141 entries), and `.evolve/instincts/lessons/`
(72 files) were **gitignored** — never in version control. Their durable content is preserved as
synthesized narrative under `knowledge/evolution/` and `knowledge/incidents/`, not as raw dumps.
The bundle captures committed/branch state; the synthesized knowledge captures the *lessons*.
