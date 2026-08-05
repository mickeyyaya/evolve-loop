# Cycle 1345/1346/1348 — new evals born ignored: `GIT_STAGE_FAILED` batch HALT

> 2026-08-05. Identical-fingerprint breaker (`ship|unknown|96f17cfe3dfe`, 3× ceiling) halted the batch at cycle-1348. Fixed console-first the same evening. Companion pin: `go/internal/phasecoherence/gitignore_birth_test.go`.

## Issue

Three cycles (1345, 1346, 1348 — the `batch-review-f3-resolution-note` / eval-materializing lanes) failed identically at atomic-ship:

```
ship: git add failed (rc=1): The following paths are ignored by one of your .gitignore files:
.evolve/evals
```

The ADR-0072 identical-fingerprint breaker correctly stopped the batch: three identical failure identities cannot be distinct honest defects.

## Gap

A **tracked-corpus / ignore-ladder split**, invisible to every existing guard:

- `.evolve/evals/*.md` has been a **tracked corpus since 2026-06-01** (54 files on main), and eval-materialization lanes carry new evals as *deliverables* — the operative design says evals ship.
- But the `.gitignore` re-include ladder (which carves profiles, phases, inbox, plugin out of the blanket `.evolve/*`) **never got an evals entry**. The cycle-176 revert (issue-#11) had removed a blanket `!.evolve/` un-ignore and left the already-tracked evals in place — corpus tracked, births ignored, for two months.
- The July ship hardening (`dropIgnoredPaths`, cycle-1101) could not catch it: `git check-ignore` reports a directory containing tracked files — and the tracked files themselves — as **not ignored** (tracked content is exempt), so the pathspec survived the filter. `git add -A -- .evolve/evals` then traversed the directory, hit the *untracked new* eval matching `.evolve/*`, and refused the whole pathspec — rc=1, deterministically, on every retry.

One sentence: **check-ignore answers "is this path ignored?", but `git add` fails on "does this pathspec sweep any ignored file?" — a tracked-corpus dir with ignored births passes the first and fails the second.**

A second latent instance of the same class existed at `.evolve/inbox-parked/` (tracked parked queue items, ignored births).

## Solution

1. **Ladder carve-outs** (`.gitignore`): `!.evolve/evals/` + `.evolve/evals/*` + `!.evolve/evals/*.md`, and the same shape for `.evolve/inbox-parked/*.json` — the exact idiom the phases/profiles carve-outs use. The issue-#11 NOTE is updated in place: the "deliberate redesign" it demanded has since happened piecemeal (recoverBuildLeak skips `.evolve/`, `dropIgnoredPaths`, eval-quality gates); the scoped ladder line was the missing final piece. Blanket `!.evolve/` remains forbidden.
2. **Class pin** (`go/internal/phasecoherence/gitignore_birth_test.go`, runs in the ship-time repo-contract pack): for every `(directory, extension)` pair in the tracked corpus under `.evolve/`, a NEW birth must be possible in at least one of the two shapes the ladder uses — a new name beside the exemplar (evals) or the exemplar's exact name in a new sibling directory (phases). Designed-ignored surfaces (`inbox/processed/` legacy, `instincts/lessons/` `.keep`-only, the `.evolve` root's explicit whitelist) are allowlisted with reasons. RED on the pre-fix tree reported exactly the two genuine violations and zero false positives.
3. **Verified against the halt shape**: an untracked `.evolve/evals/__halt-repro__.md` + `git add -A --dry-run -- .evolve/evals` refuses before the fix and stages cleanly after; `.evolve/state.json` stays ignored (anti-regression both directions).

## Why the breaker, not the gate, caught it

`dropIgnoredPaths` fails *open* by design (the probe must never block ship), and its probe primitive is honest — the gap was that probe's question being subtly different from `git add`'s. The breaker's identical-fingerprint ceiling is what converted a silent per-lane retry burn into a diagnosable system halt with preserved worktrees — exactly its job. The recovery path (`evolve loop --reset --fingerprint` + fresh-boot binary self-heal) then deploys this fix.
