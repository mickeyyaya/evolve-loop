# Failure Identity & Carryover Lifecycle

> Cycle-1440. Covers the three seams that decide **what counts as the same failure**
> and **when a remembered failure is done**: carryover retirement, staging-refusal
> classification, and reason fingerprint normalization.

Three subsystems answer one question — *is this the same defect I already saw?* —
and each had a leak that made the loop burn cycles re-working closed items.

## 1. Carryover retirement (PASS closeout)

`state.json:carryoverTodos[]` is the cross-cycle memory of open follow-ups, and the
router prompt renders its first 20 entries. Before this change `mergeCarryoverTodos`
(`go/internal/core/failure_learning.go`) only ever **unioned** disk with incoming and
deduped by ID — nothing ever removed an entry. An entry whose work actually shipped
persisted forever; the 2026-08-10 investigation found 124 of 254 live entries were
stale duplicates of a handful of classes, so the router window was mostly done work.

**Seam:** `core.RetireCarryoverTodos(todos, committedIDs) []CarryoverTodo`.

An entry retires when either:

- its `ID` is in `committedIDs` (the ids this cycle actually committed to), or
- it shares a retired entry's **cross-cycle Action fingerprint**
  (`carryoverActionFingerprint` — Action text with cycle tokens folded and
  whitespace/case normalized). These are the per-cycle re-mints of the same class
  that the ID-keyed dedupe never collapsed.

Everything else survives in original order (the router window is ordered), and the
input slice is never mutated — callers re-read state under a lock, so a mutated input
would corrupt a concurrent peer's merge. A blank committed id is malformed input, not
a claim about the equally-malformed blank-ID entry, and retires nothing.

**Production caller:** `retireCommittedCarryover` in
`go/internal/phases/ship/postship.go`, invoked from `promoteInbox` inside the landed
branch. Two properties are load-bearing:

- It rides the **same landing gate** as inbox promotion (cycle-598). Retiring on an
  unlanded ship would erase the only record of work that never shipped.
- It is **bookkeeping only**: a missing `state.json`, an unreadable state map, or a
  failed write is a `WARN` in `res.Logs`, never a ship error.

It rewrites the *raw* JSON entries rather than round-tripping them through the typed
struct, so a field this binary does not model survives retirement, and it takes the
same `withStateLock` every other `state.json` RMW takes (ADR-0049 S2 / G2).

## 2. Two-strikes staging refusal

`stageExplicitPaths` (`go/internal/phases/ship/gitops.go`) classified **every**
refused `git add` as `core.ShipClassTransient`, so the failure floor kept
re-dispatching refusals that could never succeed in place. Cycle 1365 burned its
entire retry budget on one `.evolve/evals` pathspec: its worktree base predated the
`.gitignore` carve-out, so no retry could ever win.

**Rule:** the *first* refusal of a given pathspec stays `ShipClassTransient` — a
genuinely flaky add must keep its retry. A *second consecutive* refusal of the
**same pathspec** reclassifies to `ShipClassPrecondition`: deterministic, so the
router routes to continuation/salvage instead of another doomed attempt.

- Identity is the **sorted path set** (`stageRefusalKey`), so "same pathspec" is
  order-independent but a different refused path is a different failure and resets
  to transient. A rule that merely counted refusals would wrongly kill the retry for
  an unrelated second failure.
- Memory is **workspace-scoped** (`<workspace>/ship-stage-refusal.txt`): fleet lanes
  run concurrently, so one lane's strike must never block a peer's first attempt.
- With **no workspace** there is nowhere to record a strike, so classification
  degrades to the pre-existing transient behavior rather than guessing deterministic.
- A successful staging call clears the memo, so *consecutive* means consecutive.

## 3. Reason fingerprint normalization

`normalizeReasonForFingerprint` (`go/internal/core/failure_digest.go`) projects a
reason onto its **defect identity** for the identical-fingerprint breaker
(`IdenticalFingerprintCeiling = 3`). Display and identity are two projections of one
reason string: the digest, the dossier and `audit-fail-reason.json` all keep the
reason verbatim — only the hash input is normalized.

It previously stripped two tokens (`narrative=<verdict>`, go-test durations). Every
other per-cycle-varying token still split ONE recurring defect into N fingerprints,
so the breaker never reached its ceiling and the batch kept burning cycles. Two
further tokens now fold:

| Token | Shape | Why |
|---|---|---|
| `cycleNumberToken` | `.evolve/runs/cycle-1365/…`, `.evolve/worktrees/cycle-42824668-1440/…`, bare `cycle 1365` | The same abort recorded on two cycles is one defect. The trailing `(?:-\d+)*` is required for the worktree shape, whose name carries both a lane hash and a cycle number. |
| `attemptDenominatorToken` | `(attempt 1/3)`, `retry 3 of 4` | One unwinnable defect retried N times is one defect — exactly the retry storms the breaker exists to halt. |

**Blast radius is the constraint, not the folding.** Over-normalization that
collapsed two different defects would blind the breaker far worse than the variance
it fixes, so:

- Only the *number* folds — the path around it (which directory, which artifact
  **file**) is untouched, so two artifacts in one cycle dir stay two defects.
- The attempt keyword is preserved via `${1}`, so an `attempt` reason and a `retry`
  reason — different writers, different failures — cannot collapse into each other.
- The pre-existing `narrative=<verdict>` and duration pins stay green.

The negative tests (`DistinctDefectsStayDistinct`,
`TouchesOnlyTheNarrativeToken`, `DifferentPathspecStaysTransient`) are what refute a
degenerate implementation: a normalizer returning a constant, or a strike rule that
merely counts, satisfies every positive test above.

## Acceptance coverage

`go/acs/cycle1440/predicates_test.go` exercises each seam by running the named tests
in the production packages — never a source grep, which would pass on a cosmetic edit
that normalizes nothing. `TestC1440_002_CarryoverRetirementWiredIntoPassCloseout` is
the one that actually gates §1: it drives `promoteInbox` end to end and asserts the
observable `state.json` mutation, because a seam whose only caller is a test is dead
code.
