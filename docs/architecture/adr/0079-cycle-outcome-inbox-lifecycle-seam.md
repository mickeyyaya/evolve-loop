# ADR-0079 — One cycle-outcome seam for the inbox lifecycle

- **Status:** Accepted
- **Date:** 2026-07-28
- **Cycle:** 1156
- **Supersedes/extends:** ADR-0072 S5 (task-level retry ceiling + quarantine)

## Context

The inbox lifecycle had two half-implementations and a hole between them.

**PASS side.** Promoting a shipped item to `processed/cycle-N/` was driven by a
prose instruction in the ship phase's agent prompt plus a `for` loop in
`promoteInbox` that only ran when the whole landing gate passed. Cycle-1147
shipped three menu items in ONE commit; `processed/cycle-1147/` was empty
afterwards and all three items were re-offered by the very next triage. Work
that shipped kept re-entering the backlog (`menu-pass-promotes-committed-ids`).

**FAIL side.** `ReleaseCycleProcessingWithQuarantine` is the only caller of
`bumpFailureCount`, and it walks `processing/cycle-N/` only. Nothing ever put a
wave lane's worked ids there — lanes read their scope from `lane-scope.json` and
triage builds its menu from the inbox ROOT — so the ADR-0072 S5 retry ceiling
was structurally unreachable for fleet work. Batch-14 burned four consecutive
FAILs (1137/1139/1142/1143) on the same items with `failure_count` never leaving
0 (`wave-lane-task-quarantine-dead`).

**Non-delivery reported as success.** `Promote` treated a destination `MkdirAll`
failure as `(NoOp: true, nil)` — the ship.sh "source already moved" compat
contract — so a stranded task was indistinguishable from a completed one to
every caller, and `evolve inbox-mover promote` exited 0
(`inboxmover-promote-mkdir-fail-loud`).

## Decision

**1. One seam, both verdicts.** `inboxmover.ApplyCycleOutcome(opts, CycleOutcome)`
is the single entry point for applying a cycle's verdict to the inbox lifecycle.
Both closeout paths call it:

| Path | Call site | Verdict |
|---|---|---|
| PASS closeout | `internal/phases/ship/postship.go` `promoteInbox` | `Passed: true` |
| FAIL closeout | `cmd/evolve/cmd_loop.go` cycle-level-failure branch | `Passed: false` |

PASS promotes exactly the committed ids to `processed/cycle-N/` and then drains
residual claims. FAIL bumps `failure_count` on exactly the committed ids and
quarantines any that reach `Ceiling`, releasing everything else untouched.

**2. The committed set is the unit of accounting, not the menu.** `CommittedIDs`
(the union of `top_n[].id` and `skip_shipped[].task_id`) is read by ONE function
that both paths share. A wave lane is offered a menu but works only what triage
committed; bumping the whole `processing/` dir would quarantine healthy backlog
after N failures of an unrelated task. `nil` committed ids preserve the legacy
whole-dir behavior, so an absent or unreadable `triage-decision.json` degrades to
exactly today's semantics rather than silently bumping nothing.

**3. Claim at outcome, not at dispatch.** `ClaimLaneScope(opts, cycle, ids)`
moves resolvable ids into `processing/cycle-N/`, tolerating ids it cannot
resolve. It is invoked from `ApplyCycleOutcome`'s FAIL path, **not** at wave
dispatch: triage builds its menu from `inboxbatch.LoadDir` on the inbox root
only (`triage.go:113`), so claiming a lane's scope before triage runs would hand
triage an empty inbox and starve the very cycle the claim exists to track.
Claiming at outcome time puts the worked ids where the drain needs them, with no
starvation window. `ClaimLaneScope` is exported so a future dispatch-side caller
(one that also teaches the menu readers about `processing/`) can reuse it.

**4. A failed move is an error.** `Promote` returns `ErrMvFailed` with
`NoOp == false` when the destination `MkdirAll` fails, leaves the item where it
was, and keeps its `promote-warn` ledger line. `evolve inbox-mover promote` maps
that to exit 2, matching `claim`'s mv-failed code. The `rename` failure path and
the genuine "source already moved" no-op are unchanged — the compat contract
still covers the case it was written for, and only that case.

## Consequences

- The PASS/FAIL halves can no longer drift apart: they share one function, one
  committed-set reader, and one drain core.
- The S5 retry ceiling is reachable for fleet work for the first time; a poison
  todo now stops being re-picked after `TaskRetryCeiling` real failures.
- A system-level (S3) failure still never quarantines — `ShouldQuarantine`'s
  precedence rule is unchanged and now flows through `CycleOutcome.SystemLevel`.
- Callers that previously read `Promote`'s `NoOp` as "fine either way" now see an
  error on infrastructure failure. `ReconcileSuperseded` propagates it (loud by
  design); the quarantine path inside the drain still fails open to a normal
  release, so a bookkeeping fault never strands an item.
- A `promote` that cannot create its destination now exits non-zero. Any script
  relying on the blanket ship.sh exit-0 contract for that specific failure will
  see the failure it was previously swallowing.

## Accepted risk — `ClaimLaneScope` mutates the shared inbox root

Decision 3 explains why the claim happens at outcome time. It did not state what
that placement costs, and the cycle-1156 audit (D4) was right to ask.

**The exposure.** `ClaimLaneScope` is called from a *per-lane* closeout path, but
it moves files out of the *shared* `.evolve/inbox/` root — the same root every
sibling lane's triage reads, with no lane isolation (`inboxbatch.LoadDir` on the
root only, `triage.go:113`). At the standing fleet width of 3 those lanes are
live concurrently, so a sibling lane whose triage runs while a FAILing lane's
claim is in flight can **miss** an item that is momentarily in
`processing/cycle-N/` rather than at the root. The blast radius is one item, for
one sibling cycle: the item is not lost, not double-worked, and not corrupted —
it is invisible to one triage pass.

**Why it is accepted rather than guarded.** The two bounding mechanisms below
already cap the window at one cycle and make the concurrent case non-destructive.
Closing it fully means a lock (or a lane-aware root reader) around a
self-healing, single-cycle, single-item window — new cross-lane locking on the
hot inbox path, whose own failure modes (a stale lock stranding the whole fleet)
are worse than the miss it prevents. The audit offered "acknowledge (accepted
risk) or guard"; this is the acknowledgement.

**Bound 1 — the residual drain self-heals it.** `ApplyCycleOutcome` always drains
`processing/cycle-N/` back to the inbox root at cycle end, on PASS and on FAIL
alike, and a residual claim released on PASS accrues no `failure_count`. The miss
window therefore closes after **one** cycle without operator action; it cannot
accumulate into a permanently invisible item.

**Bound 2 — the dest-exists double-move guard.** The drain skips an item whose
destination already exists at the root (`inboxmover.go:706-710`) instead of
renaming over it, so a concurrent release that already landed the root copy is
never clobbered by a second lane's drain. The concurrent case degrades to a
no-op, not to data loss.

**Re-evaluate if** the fleet width grows well past 3, triage learns to read
`processing/` (which would remove the exposure outright), or the drain's
unconditional residual pass stops being unconditional — the first two shrink the
risk, the third invalidates Bound 1 and reopens this decision.

Pinned by `go/acs/cycle1160/predicates_test.go` predicates 004 (this text names
the mechanism and both bounds) and 005 (the code still behaves the way this text
describes), so the prose and the behaviour cannot drift apart silently.

## Verification

`go/acs/cycle1156/predicates_test.go` — 8 predicates asserting the filesystem
end state (where each item lands, what its durable `failure_count` says) plus a
real-subprocess exit-code check. Permanent regression evals live at
`.evolve/evals/{inboxmover-promote-mkdir-fail-loud,wave-lane-task-quarantine-dead,menu-pass-promotes-committed-ids}.md`.

## Amendment (cycle 1157) — fail-loud on the consumer side

The original slice made the *producer* (`Promote`) fail loud on a destination
mkdir failure. Three consumers still mishandled that new error; all three are
corrected here, under one rule: **a non-delivery is always visible, and being
visible never costs an item its release.**

1. **Drain quarantine path** (`inboxmover.go`, `releaseCycleProcessing`) bound
   `Promote`'s error and never read it. A failed quarantine silently fell back
   to the ordinary release, so the poison item returned to the inbox root and
   the next triage re-picked the exact task the ceiling exists to park. It now
   emits one severity-marked line carrying the task id, the failure count and
   the quarantine attempt, then falls through to the release (fail-open kept —
   stranding the item in `processing/` would be the worse defect). A successful
   quarantine stays silent, so the diagnostic cannot degrade into noise.
2. **`ApplyCycleOutcome` PASS path** returned on the first failed promote, which
   skipped `releaseCycleProcessing` entirely and re-introduced the early return
   that stranded every residual claim (orphans across cycles 124/265/294/295/
   308). Promote errors are now collected, the drain always runs, and the joined
   error is returned afterwards — `errors.Is(err, ErrMvFailed)` still holds.
3. **`postship.go`** logged the outcome failure as `WARN` and then appended
   `[ship] OK: inbox lifecycle drain complete` unconditionally — a success claim
   on a run where the drain did not complete. That line now lives in the success
   branch; the failure branch says `INCOMPLETE` instead.

Additionally, `systemLevel` now gates the durable `failure_count` **bump**, not
only the quarantine decision. Bumping during a quota/infra storm walked healthy
committed ids toward `TaskRetryCeiling`, so a later unrelated task-level FAIL
quarantined a backlog that never failed on its own merits — ADR-0072 AC4 is now
honored end to end rather than half-honored at the quarantine gate.

Verification: `go/acs/cycle1157/predicates_test.go` — 5 predicates (001/002/005
are regression locks on the producer half; 003 is the drain diagnostic; 004 is
its anti-no-op twin proving the diagnostic is conditional).

## Amendment (cycle 1180) — the FAIL half reaches the wave path

Decision 1's table claimed two call sites. Only one of them was ever on a fleet
lane's code path, so the seam this ADR built was still dead for the work it was
written for.

**What was actually wired.** The FAIL call site lived in `cmd_loop.go`'s
*sequential* branch. A wave lane never executes it: `productionWaveLauncher`
launches `evolve cycle run` as a **subprocess** (`cycleRunArgs`), and the wave
branch that collects the results only counts `failedLanes` and logs `N/M lanes
ok`. `fleet.Result{Index, ExitCode, Err}` carries neither the lane's cycle
number nor its workspace, so the supervisor is structurally incapable of
applying a lane's verdict. The S5 ceiling therefore stayed unreachable for
fleet-dispatched work even after cycle-1156 — the batch-14 symptom this ADR
diagnosed was fixed in the layer that could not see it.

Two further gaps kept quarantine cosmetic even once reachable:

- `ResolveDispatchState` classified `inbox/{root,processing,processed,rejected,
  retry}` but **not** `quarantine/`. A parked todo fell through to
  `StateUnknown`, and the dispatch freshness gate fails OPEN on unknown — so the
  ceiling parked a poison item and the next wave relaunched it.
- The wave **seed** (`SelectWaveSeedMenus` → `WidenTopNToFleetWidth`) copies the
  carried-over `committed` prefix through verbatim, re-pinning an already
  consumed id into a later `lane-scope.json` (cycle-1116 re-pinned
  `tdd-topn-binding-gate` after cycle-1113 consumed it).

**Decision 5 — the FAIL closeout is an importable package with two call sites.**
`internal/cycleoutcome.ApplyFailure(FailureInputs)` is the seam; `package main`
is not importable, which is why the logic could be neither reused by the fleet
path nor pinned by any predicate while it sat inline. `FailureInputsFor` derives
the two things a caller cannot state on its own — the S5 ceiling from
`policy.json` and whether the failure was task-level — so the blame rule has one
definition (`cmd_loop.go`'s `isTaskLevelFailure` and `failedCycleCommittedIDs`
are now thin forwarders).

| Path | Call site | Applies |
|---|---|---|
| PASS closeout | `internal/phases/ship/postship.go` `promoteInbox` | `Passed: true` |
| FAIL, sequential loop | `cmd/evolve/cmd_loop.go` cycle-level-failure branch | `ApplyFailure` |
| FAIL, fleet lane | `cmd/evolve/cmd_cycle.go` `runCycleRun` | `ApplyFailure` |

The lane applies its own verdict **in-process**, where the cycle number and
workspace are local — mirroring the PASS half, which has always lived inside the
cycle process for exactly this reason. Both FAIL shapes are covered and each
applies exactly once: a cycle-level error (`core.ErrCycleLevelFailure`) and a
clean return carrying `FinalVerdict == FAIL`. A lifecycle fault WARNs but never
changes the exit code — that code is the lane's only channel to its parent.

**Decision 6 — quarantine is a dispatch state.** `StateQuarantine` joins the
lifecycle classification, so the freshness gate's default branch skips a parked
id with `consumed: quarantine` as the reason instead of failing open on it.

**Decision 7 — the seed prunes consumed carry-overs.** `SelectWaveSeedMenus`
drops committed ids resolving to `processed`/`rejected`/`quarantine` before
widening, making the plan artifact honest rather than leaning on the launch-time
gate to skip a lane that should never have been planned. `pending` and
`processing` survive (still live work) and — load-bearing — so does an id with
**no** lifecycle evidence: a prune that dropped what it cannot resolve would
starve every wave of non-inbox-backed cards.

Verification: `go/acs/cycle1180/predicates_test.go` — 4 predicates driving the
real seams over isolated temp trees (002 walks a ceiling-3 lifecycle to
quarantine; 003 is the negative half proving uncommitted menu ids and S3
system-level failures stay inert; 001 and 004 pin the two gaps above, each with
its fail-open edge asserted). Unit coverage for the new package lives at
`go/internal/cycleoutcome/cycleoutcome_test.go`; permanent regression evals at
`.evolve/evals/{wave-lane-task-quarantine-dead,wave-planner-pass-scope-prune}.md`.
