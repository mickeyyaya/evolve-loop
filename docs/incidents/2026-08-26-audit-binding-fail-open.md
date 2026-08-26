# 2026-08-26 — Ship's independent-review binding fails open across cycles (cycle-1571 H3)

## What happened

Cycle 1571 (verification wave 2) FAILed its audit — a legitimate agent-graded
FAIL with a well-formed sentinel. Ship then errored `AUDIT_BINDING_HEAD_MOVED`
with `audited=e8003a44` — **cycle 1570's** HEAD, not 1571's. The recovery path
declined repair and forced a full deep-tier re-audit of a cycle already
adjudicated FAIL: a burned lane slot. The re-audit (dispatched at xhigh)
produced the H3 finding this document records.

## The hole — two halves composing

**Producer half.** `core/phase_bindings.go` recorded the rich auditor ledger
binding (role=auditor, kind=agent_subprocess, artifact SHA, run_id) only on
`PASS|WARN`. A FAIL verdict emitted **no** binding: the FAIL was the very
thing that removed the ship gate's ability to see it.

**Consumer half.** `phases/ship/audit.go` `findLatestAudit`, when
`opts.RunID` was set and no entry matched, fell back to the newest auditor
entry from **any** run ("zero regression for legacy unstamped ledgers"). With
the producer half above, that fallback is reached on *every FAILed cycle*, and
under fleet concurrency the newest entry belongs to a sibling lane.

**Worst case (all preconditions live, not hypothetical).** If the sibling's
entry shares `git_head` with current HEAD — routine when two lanes run inside
one HEAD window — exit-code, artifact-SHA, and verdict checks all evaluate the
*foreign* artifact. A FAILed cycle ships on a sibling's PASS with no error
surfaced. Same failure shape as the 2026-05-29 "ancient bash-era auditor
entry" incident, reachable by a different route.

## Fix (fail closed at both halves)

- **Producer** (`phase_bindings.go`): FAIL now records the same rich binding,
  `exit_code=1` (Unix findings convention — the auditor process completed;
  severity lives in the bound artifact, so ship's `0|1` exit gate passes and
  the verdict parse of THIS run's report returns the honest terminal
  `AUDIT_BINDING_VERDICT_FAIL` instead of a foreign `HEAD_MOVED`). FAIL is
  ledger-bound but **never** projected into the verdict cache (its consumers
  were designed against PASS|WARN content; a FAILed tree re-audits).
- **Consumer** (`audit.go`): a run-scoped lookup miss is now a hard
  `AUDIT_BINDING_NO_AUDITOR` integrity stop naming the refused foreign entry.
  The old fallback pin `TestFindLatestAudit_RunIDNoMatch_FallsBackToLatest`
  was deliberately FLIPPED: its unstamped-ledger premise is dead (every
  current recorder stamps `run_id`; verified against the live ledger).

- **Third reader** (review sweep finding): the composition snapshot's
  `latestAuditEntry` (`cmd/evolve/cmd_composition_wiring.go`) — the RUNG 0
  trivial-rebase carry-forward's ledger reader — had the same unscoped
  "latest auditor entry" shape, previously safe only via
  `findCompositionVerdict`'s LaneAuditRef hash cross-check. Now run-scoped
  with the identical contract (runID set ⇒ exact match or error), because the
  producer fix makes FAIL entries visible to it for the first time.

## Lessons

1. A verdict is only as enforceable as the *binding* that carries it to the
   gate; excluding the failure verdict from the binding path inverts the gate.
2. Cross-run fallbacks in integrity lookups are fail-open by construction;
   "zero regression" compatibility clauses must carry an expiry condition.
3. The escalation's canned `repro_hint` was wrong for the sixth consecutive
   time; the re-audit's artifact-first decomposition found the real hole.
