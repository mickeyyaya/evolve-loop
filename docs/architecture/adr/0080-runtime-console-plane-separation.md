# ADR-0080: Runtime/Console plane separation — the loop owns its checkout

- **Status**: Proposed (operator-requested design, 2026-07-28)
- **Deciders**: operator + console session
- **Context cycles**: 1121/1122, 1149/1150 (tree-diff guard kills from operator edits), 1151/1152 (dispatch death from operator `git rm` of runtime-minted stubs), PR #370 stowaway reds (ship auto-stage adopting operator-independent dirt)

## Problem

The main checkout plays three roles at once:

1. **Loop runtime home** — `.evolve/` state, tree-diff guard baselines, ship
   source (waves ship from this tree), phase/profile registry, pinned binary.
2. **Operator console workspace** — branch edits, staging, checkouts, pulls.
3. **Integration target** — the local `main` lanes base from.

Any operator activity in roles 2–3 mutates the working tree role 1 treats as
authoritative. Live-observed interference classes, each with a killed lane:

| Class | Mechanism | Live instance |
|---|---|---|
| W1 dirt attribution | tree-diff guard blames phase for operator edits | 1121/1122, 1149/1150 |
| W2 disk deletion | dispatch reads a runtime-minted file the operator removed | 1151/1152 |
| W3 staging races | ship auto-stage adopts operator dirt; commit-gate STALE | PR #370 stubs; repeated STALE re-gates |
| W4 HEAD switches | operator checks out a branch in the loop's tree | batch-12 near-miss |
| W5 stale local main | dossier commits diverge local vs origin; lanes base stale | recurring pull conflicts; queued `loop-must-base-lanes-on-origin-main-not-stale-local` |
| W6 config mutation | control-plane files change mid-batch outside preflight | `.apicover-enforce` enrollment churn |

Discipline-based mitigation (the `live-loop-console-dev-worktree-only` memory
rule) reduces W1/W2 but is a policy, not a structure: it failed twice in one
session. The fix must make interference **impossible by construction**.

## Decision

Adopt a **bare-hub, role-worktree** layout and give the loop a dedicated
checkout the operator never touches:

```
~/ai/claude/evolve-loop.git           # bare hub: shared objects + refs, no working tree
~/ai/claude/evolve-loop/              # CONSOLE worktree — human/operator, any branch
~/ai/claude/evolve-loop-runtime/      # RUNTIME worktree — loop-owned, branch runtime/main
   └── .evolve/…                      # runtime state (untracked ⇒ naturally per-worktree)
   └── .evolve/worktrees/cycle-*/     # lane worktrees, unchanged (hang off the shared hub)
```

1. **The loop launches with project-root = the runtime worktree.** Everything
   already keys off `ProjectRoot`/cwd, so the tree-diff guard, ship source,
   registry reads, `.evolve` state, binary and SELF_SHA pin all move with it.
2. **Branch policy.** The runtime worktree pins `runtime/main`, fast-forwarded
   from `origin/main` ONLY by the loop at wave boundaries (existing sync-main
   machinery, merge-only). The console keeps `main` + feature branches. Git's
   same-branch dual-checkout restriction then *enforces* the plane split
   instead of fighting it. Integration happens exclusively via origin: lane
   ships push from runtime; console changes merge via PR. This subsumes W5 and
   the queued stale-base item — lanes always base on a fresh origin image.
3. **The inbox is the one sanctioned cross-plane channel.** Console files
   items with `--project-root <runtime>` (the CLI already accepts it) or the
   `EVOLVE_RUNTIME_ROOT` convenience env. Inbox drops are append-only atomic
   JSON writes with no index/worktree interaction — the safe-anytime contract
   survives unchanged. All other console→runtime influence flows through
   origin merges picked up at boundary syncs.
4. **Guard precision dividend.** With the operator structurally out of the
   runtime tree, ANY tree-diff dirt is a real leak: the guard gets stronger,
   with zero false kills, rather than being bypassed or allowlisted.
5. **Operator intervention path unchanged in shape**: console-first pipeline
   fixes build in the console worktree → PR → origin; the runtime picks them
   up at a boundary bounce (rebuild + `reset-sha` inside the runtime tree).
   Emergency stop remains the wave-boundary SIGINT.

## Shared-store hazards (research-encoded operating rules)

Worktrees share the object database and refs; each has its own HEAD + index.

- Never `git gc --aggressive` while lanes are live; check
  `git worktree list -v` for prunable entries and `git worktree repair`
  before ANY aggressive cleanup.
- `git worktree prune` only at batch boundaries (a manually deleted lane dir
  leaves a stale registration that blocks its branch).
- All worktrees on local disk — network filesystems break git's locking.
- Lane branch names stay unique (already true), so shared-ref updates do not
  contend.

## Migration plan (each slice independently shippable)

- **S1 (zero code)**: convert the clone to the bare-hub layout
  (`git clone --bare` + two `git worktree add`s, or in-place: move `.git` to
  the hub and re-add both worktrees); launch the loop from the runtime
  worktree; run one full wave as the soak. Rollback = launch from the console
  path again.
- **S2 (small code)**: `evolve doctor` runtime-isolation probe — WARN when the
  loop's project-root working tree shows operator-shaped activity (HEAD
  switched mid-batch, index mtime newer than the batch's own operations);
  docs updates (runtime-reference layout section, this ADR).
- **S3**: boundary sync hardening — re-point sync-main at
  `origin/main → runtime/main` fast-forward; retire the queued
  `loop-must-base-lanes-on-origin-main-not-stale-local` item against it.
- **S4 (defense in depth, optional)**: console-activity ledger consulted by
  the tree-diff guard for the residual case of someone editing the runtime
  tree anyway.

## Alternatives considered

- **Guard allowlist / operator lease** (declare intended-dirty paths; guard
  subtracts them): keeps one checkout but fixes only W1's attribution — W2,
  W3, W4 remain, and every leased path is a guard blind spot. Rejected:
  patches the symptom, weakens the invariant.
- **Full second clone**: same isolation, but duplicates the object store,
  slows lane provisioning (lanes currently share objects for near-free
  worktree creation) and adds ref-drift risk. The bare hub gives identical
  isolation with one store.
- **Containerized runtime** (ephemeral-clone platforms in the style of hosted
  coding agents): the strongest isolation, but heavyweight for a single-host
  loop whose lanes already sandbox subprocesses (`EVOLVE_SANDBOX=1`). Noted
  as the natural next step if the loop ever becomes multi-host.

## Consequences

- Operator errors of the W1–W4 classes become structurally impossible, and
  four standing session-memory rules collapse into one layout fact.
- The console loses the ability to hotfix the runtime tree in place — by
  design; the PR → boundary-bounce path is the only write channel, making
  every runtime change attested and CI-proven.
- Disk cost: one extra full checkout (~repo size, shared objects).
- The `evolve` CLI needs no immediate change (S1 is pure operations); S2/S3
  are small, testable slices.

## References (research, 2026-07-28)

- Bare-repo + worktree layout and sibling-directory conventions:
  [gitworktree.org best practices](https://www.gitworktree.org/guides/best-practices),
  [bare-repo setup guide](https://www.gitworktree.org/guides/bare-repo),
  [xoofee: professional worktree workflow with a bare repository](https://xoofee.github.io/posts/2026/04/git_worktree_bare_repo_workflow/)
- Agent isolation + merge-queue integration discipline:
  [ctx.rs: why coding agents need a merge queue](https://ctx.rs/blog/merge-queue-for-agents/),
  [claude-code-merge-queue (local queue for parallel agents)](https://github.com/funador/claude-code-merge-queue)
- Worktree sharp edges (same-branch checkout, prune/repair, gc, refs sharing,
  local-disk requirement):
  [git-worktree official docs](https://git-scm.com/docs/git-worktree),
  [FixDevs: worktree failure modes](https://fixdevs.com/blog/git-worktree-not-working/),
  [Mastering git worktrees](https://martinuke0.github.io/posts/2026-03-27-mastering-git-worktrees-a-complete-guide-for-developers/)
