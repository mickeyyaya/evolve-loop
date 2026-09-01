# 2026-09-02 — repair round inherits the superseded acs-verdict; diagnosed downgrade halts as "forgery"

## Impact
Cycle 1603 (wave-20260902, lane 3): a fully successful ADR-0092 audit repair —
round-1 audit FAILed on substantive findings (H1 propagation gap + M1), tdd/build
fixed them, round-2 auditor re-engaged H1 and declared PASS — was recorded FAIL,
routed to retro, prose-classified `verdict-incoherence`, and HALTED the batch at
SYSTEM level (rc=4, P0 inbox item, pipeline-escalation.json). The wave ended 0/3.
The repair loop was **structurally unable to ever succeed** in this shape: every
round re-read round-1's verdict file, so each repair's PASS was force-downgraded
right back to FAIL.

## Root cause (two defects, one chain)
1. **Stale verdict artifact across repair rounds.** The auditor persona pre-writes
   `acs-verdict.json`, and `audit.Classify` honors a pre-staged file (the
   verdict-exists gate skips `generateACSVerdict`). Round-1's auditor amended the
   file with `ship_eligible: false` + `audit_verdict: FAIL` extras (01:45).
   On repair re-entry nothing retired that file, so round-2's audit (03:19)
   inherited it, and the EGPS override (`audit.go` "EGPS ship_eligible=false"
   case) forced the fresh narrative PASS to FAIL — correctly, per its own
   contract, but against evidence from a superseded round.
2. **Prose outranked deterministic evidence.** The forced FAIL carried a
   persisted substantive reason (`cs.AuditFailReasons` via
   `persistFloorFailReasons`), so the deterministic incoherence detector
   correctly declined to fire (`coherence.CheckVerdictCoherence`'s
   `SubstantiveError` guard: a diagnosed downgrade is a justified negative
   verdict, not a forgery). But `applyFailureDecisionFloor` path (2) then halted
   on the retro agent's `failure-decision.json` claiming `verdict-incoherence` —
   prose asserting the exact category the deterministic layer had already
   examined and rejected.

## Fix (PR: fix/host-stamps-acs-audit-binding)
1. `core/audit_round_artifacts.go` — `retireSupersededAuditArtifacts` renames
   the superseded round's verdict artifacts to round-suffixed archives
   (`acs-verdict.round<N>.json`, `audit-report.round<N>.md`) so the fresh audit
   regenerates from execution, while the old evidence survives for forensics
   (the cycle-1434 `acs-verdict.foreign.json` precedent). Wired at the audit
   PRE-DISPATCH seam beside `resetFloorFailReason`, mirrored on both dispatch
   surfaces (`cyclerun_dispatch.go` + `resume.go`) — the seam that already owns
   "a re-dispatch supersedes the prior attempt", so it covers EVERY re-audit
   path (repair loop, bookkeeping regrade, ship-error recovery, RERUN_PHASE),
   not just the repair branch, and opens no artifact blackout window across
   the re-entered tdd/build phases. Round index = the persisted
   `CycleState.AuditDispatches` counter (advanced by the ONE primitive
   `supersedePreviousAuditRound`, incremented pre-dispatch BEFORE the
   cycle-state write, so a round that crashes or quota-pauses mid-flight has
   already recorded its dispatch and its dead attempt's verdict is retired on
   resume — review-2 HIGH), floored by the completed-audit count in
   `CompletedPhases` for pre-field legacy checkpoints. Round 0 retires
   nothing, so an operator/CI pre-staged verdict keeps the honor
   `audit.Classify` grants it. (First cut hung the retirement on the
   repair-grant primitive; the architecture review BLOCKED it — wrong seam,
   subset coverage, blackout window — and the pre-dispatch placement + the
   dispatch-persisted index are the applied fixes.)
2. `core/decision_branch.go` — path (2) treats a prose `verdict-incoherence`
   claim as CONTRADICTED (loud stderr WARN, task-level FAIL) when
   `hasSubstantiveFailReasons(cs)` holds — the SAME single-sourced predicate
   (`core/system_failure.go`) that feeds `SubstantiveError` at both coherence
   surfaces, so the prose gate and the deterministic detector can never drift.
   The forgery halt is preserved for the undiagnosed case (converse pinned).
3. `acssuite.VerdictFilename` — first exported spelling of the acs-verdict
   filename; the remaining literal copies are queued for a sweep.

## Lessons
- A repair loop that re-enters earlier phases must define which artifacts are
  ROUND-SCOPED and retire them at re-entry; "regenerate when absent" gates turn
  any surviving stale artifact into replayed state.
- When a deterministic detector owns a category and has affirmatively declined,
  a prose classification of the same category is contradicted evidence, not
  corroboration. (Same family as the wave-3 disposition narrowing, cycles
  1572-1574.)
- The halt evidence quoted both artifacts' green fields but not the persisted
  fail reason that explained them — the reason existed and was simply not
  consulted by the prose gate.

## Regression coverage
See REGRESSION-COVERAGE-INDEX.md row for this incident.
