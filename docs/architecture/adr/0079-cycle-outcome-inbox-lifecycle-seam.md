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
