# The push-strand class: console merges vs a live batch
**Period:** 2026-08-03 → 2026-08-05 · **Status:** dance documented; class fix queued (`ship-push-only-recovery`, 0.8)
**Primary artifacts:** runtime plane merges `d2f615c2` `1ce3cb2f` `c74295ff` · consumed P0 `inbox/consumed/2026-08-04T06-40-00Z-pipeline-defect-pipeline-blocker.json` · `docs/operations/operating-policy.md` §3.9 context

## Problem
The runtime plane (a linked worktree holding branch `main`, where the loop
ships) and the console (which merges PRs into `origin/main`) are two writers
against one branch. When a console PR merges while the plane holds
**unpushed lane-ship commits**, `origin/main` and plane-`main` truly diverge:
every subsequent lane push fails `GIT_PUSH_REJECTED` ("not possible to
fast-forward"), ships strand locally, PASS work goes unpublished
(the 948-class), and — as happened live — three identical ship failures trip
the identical-fingerprint breaker and **halt the whole batch** (cycle-1286).

## Context & evidence
- Ship's own policy refuses auto-rebase: *"audited tree must be re-audited on
  the new base (no auto-rebase; local commit preserved)"* — correct for audit
  integrity, but it converts divergence into a hard stop.
- Instance 1 (relaunch morning): pre-existing strand of dossier commits +
  queue ship, resolved by the sync dance; instance 2: PR #408's merge
  stranded **16 commits including the cycle-1283 CRITICAL fix**; every lane
  push failed until reconcile; the accumulated identical `ship|unknown`
  fingerprints halted the batch at cycle-1286 (see the consumed P0's
  `consumed_by` field — the canned "forged verdict" text did not apply).
- Earlier same-day trains (#404–#407) were absorbed **only because the plane
  happened to be fast-forwardable at merge time** — the hazard is
  conditional, which is why it looked safe four times before it wasn't.
- An add/add conflict variant exists: a lane and a console PR independently
  adding the same doc path (the batch-integrity review doc) makes even
  `sync-main` refuse; resolution requires a manual merge.

## Approaches considered
1. **Never merge console PRs mid-batch** — the original standing rule; too
   coarse now that console fixes are often the *unblocking* work for the
   batch itself (this window's #402/#404 fixed main so lanes could ship).
2. **Auto-rebase lane ships onto the new origin** — rejected: violates the
   audited-tree invariant (a rebase invalidates the tree SHA the audit bound).
3. **Sync-plane-immediately-after-every-console-merge** (postcondition
   discipline) — adopted operationally; applied after #409 with zero strand
   window.
4. **A push-only recovery command** usable mid-batch (publish already-audited
   local commits after reconcile without re-audit) — the queued class fix.

## Decision & reasoning
Operational discipline now, mechanism later. The reconcile dance, verified
twice end-to-end:

1. Land the plane's dirty queue state first (`git add .evolve/inbox/` →
   commit-gate → `evolve ship --class manual`; the push is *expected* to
   reject — the commit is the point).
2. `evolve sync-main` (merge-only). On an add/add conflict it refuses
   cleanly; resolve manually (take the superset), then `git merge --continue`
   — which passes the ship guard where a bare `git commit` is denied.
3. Nothing pushes yet: the **next lane PASS ship fast-forwards origin and
   publishes the whole backlog** (observed: 16 commits flushed by one ship).

The trailing hazard — origin advancing again between merge and the next lane
ship — is why step order matters: run the dance immediately after the train
reports MERGED, not at leisure.

## Implementation
No code shipped for the class yet (deliberately: the fix shape needs design —
a push-only command must prove the pushed commits' attestations still bind).
Shipped instead: the dance in this entry + the P0 consumption record; §3.9's
diff-derived-ledger rule (the halt's canned text mis-attributed the cause to
verdict forgery — the true cause lived in git topology); evidence lines on
`ship-push-only-recovery` (raised to 0.8) naming the fix shapes:
*sync-plane-before-console-merge as a train postcondition* OR *a mid-batch
push-only recovery command*; plus a fingerprint-vocabulary note — ship
failures fingerprint as class `unknown` and should carry the ship error code
(`GIT_PUSH_REJECTED`) so the breaker names the class it is counting.

## Results (measured)
- Instance-2 recovery: 16 stranded commits published by the first post-
  reconcile PASS ship; batch relaunched; zero further push rejections across
  the following ~10 cycles (monitor greps `GIT_PUSH_REJECTED` continuously).
- Post-#409 application of the postcondition: backlog never exceeded the
  in-flight dossier; no strand, no halt.

## Retrospective — what we learned
- **Two writers on one branch is a topology problem; no amount of verdict
  hygiene fixes it.** The breaker did its job — the fingerprints were
  genuinely identical — but the P0's canned diagnosis ("forged verdict") was
  wrong, which is exactly why §3.9 demands root-cause text derive from the
  actual failure, not the template.
- **Conditional hazards are the dangerous ones**: four safe merges taught the
  wrong lesson; the fifth halted the batch. "It worked last time" is evidence
  about last time's FF-ability, nothing more.
- The guard architecture held under pressure: bare `git commit` denied,
  `git merge --continue` sanctioned — the recovery never needed a bypass.
- `[session-evidence]` Counts (16 commits, cycle numbers) are from the live
  session log and the consumed P0 record.

## Links
[2026-08-release-engineering.md](2026-08-release-engineering.md) ·
[2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md) (§3.9) ·
`docs/operations/operating-policy.md` · queue item `ship-push-only-recovery`
