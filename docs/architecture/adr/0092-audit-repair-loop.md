# ADR-0092 — An audit rejection is repairable in-cycle, and prose alone may no longer outrank the deterministic floor

- **Status:** Accepted (2026-08-27) — repair defaults ON at 2 attempts
  (`policy.DefaultMaxAuditRepairAttempts`); `workflow.max_audit_repair_attempts: 0`
  disables it.
- **Driving evidence:** wave 3, cycles 1572/1573/1574 (2026-08-27). Three lanes spent
  **367 minutes** to deliver three audit FAILs and **zero ships**. Two of the three root
  causes were the *staged index contents* rather than the code: 1572 had 10 green ACS
  predicates and `red_count == 0` for its product change and was rejected for an
  out-of-lane phase stub riding the shipping index; 1573 was rejected for a build report
  declaring zero changed files against a bound tree of 10. Each rejection paid a full
  cycle teardown to fix what a rebuild could have addressed in place.
- **Related:** [ADR-0072](0072-system-failure-policy.md) — the floor this narrows in
  exactly one place, and preserves everywhere else.
- **Related:** [ADR-0076](0076-continuation.md) — the cross-cycle findings handoff whose
  reader (`readContinuationFindings`) this reuses rather than duplicating.
- **Composes with:** `docs/incidents/2026-08-12-proxy-as-verdict-findings.md` — the class
  this is an instance of: agent prose treated as a machine verdict.

## Problem

An audit FAIL was **terminal for the cycle**. `statemachine.go` gives `PhaseAudit` exactly
two successors — `PhaseShip` on PASS/WARN, `PhaseRetro` on FAIL — with no edge back to
Build or TDD. The advisor was never re-consulted: the only replan hook is
`postScoutReplan()`, fired solely when the next phase is scout. Wave-3 timestamps confirm
it — replan ran at 01:07 in cycles 1573/1574, an hour before audit at 02:24–02:54.

The repair machinery was *almost* all present and simply never reachable for this case:

- `PhaseRetro: {PhaseShip, PhaseTDD, PhaseEnd}` was already legal, and `recoveryTarget`
  already resolved control-phase recovery targets from config.
- `failureadapter.Decide` returns `ActionRetryWithFallback` from exactly one rule —
  `InfraTransient > 0` under Strict, an EPERM/sandbox retry. It has **no** notion of
  "the audit rejected the work, go fix it".

And a second defect kept even a correct repair from firing. `applyFailureDecisionFloor`
halts on either of two gates: (1) the deterministic `FloorCandidate`, and (2) the
agent-authored `failure-decision.json` category. All three wave-3 cycles recorded
**`floor_candidate: ""`** — gate 1 never fired. Each halt came *solely* from gate 2, while
the **same retro agent's** `disposition.json` recorded **`legitimacy: "legit-rejection"`**
in the same cycle. One agent contradicted itself, and the prose half halted three cycles
that two deterministic signals called task-level.

## Decision

**1. A task-level audit rejection re-enters `tdd → build → audit`, bounded at 2 attempts.**
The branch is taken in `decideAfterRetro`, below the floor and below the bookkeeping
regrade (a cheaper, more targeted re-dispatch wins where it applies) and above the
adapter. It routes through `recoveryTarget(PhaseRetro, "REPAIR_RETRY", PhaseTDD)`, so the
destination stays config-selected with `CanTransition` as the legality constraint. **No new
edge is added to the transition table** — an `Audit→Build` edge was rejected precisely
because it would bypass the retro chokepoint where the floor lives.

**2. Prose contradicted by the deterministic evidence loses.** Gate 1 is untouched: a
deterministic floor candidate halts unconditionally. Gate 2 now halts unless the claim is
contradicted by **both** an empty deterministic candidate **and** the agent's own
`legit-rejection` disposition. An uncontradicted claim, an absent or indeterminate
disposition, or an exhausted budget still halts exactly as before — which is why the
cycle-1001 judgment-halt shape is preserved byte-for-byte.

**3. The audit's own reasoning reaches the agents asked to act on it.** A repair
re-dispatch seeds `PhaseRequest.Context[CtxKeyAuditRepairFindings]` from the cycle's
`audit-fail-reason.json`, and **both** TDD and Build render it, fenced as DATA rather than
instructions. Previously only Build read the sibling `continuation_findings` key at all —
measured on cycle-1577, whose scout, fault-localization and bug-reproduction prompts
carried zero references to the failure it was resuming.

## Consequences

**The floor is intact where it is load-bearing.** Only prose contradicted by two
deterministic signals is narrowed. `decision_branch_repair_test.go` pins both directions
with byte-identical inputs differing only by the presence of `disposition.json`.

**Absence of evidence never grants repair.** A missing, unreadable, indeterminate, or
out-of-vocabulary disposition is not eligible. `readDispositionLegitimacy` is the fail-SOFT
counterpart to the fail-HARD `VerifyDisposition`, and soft-failing to `""` is the safe
direction because `""` means "do not repair".

**The bound is durable, not in-memory.** `CycleState.AuditRepairAttempts` persists in
`cycle-state.json`, so the count survives the round trip *and* a crash-resume. An in-memory
counter would silently reset on resume and hand a failing cycle unlimited rebuilds — the
recurrence class the bookkeeping-regrade bound already documents. `consumeAuditRepairGrant`
is the single primitive both branch surfaces call, and a source-level guard test fails if
either surface drops it.

**Cost.** A repaired cycle pays up to 2 extra `tdd + build + audit` passes. Against wave 3
that is the trade being bought deliberately: 367 minutes produced nothing, and two of the
three rejections were addressable in place.

**Known limitation, recorded rather than fixed.** Repair eligibility depends on
`disposition.json`, which is a retro deliverable, so the first rejection still pays a full
retrospective before any repair can be decided. Making the first decision cheap would mean
deriving legitimacy without the retro agent — new inference, deferred deliberately.

**What this does not change.** Ship-time audit binding is untouched: each repair attempt
writes its own auditor ledger entry and ship binds the newest run-scoped one
(ADR-0084/#503/#504). Repair attempts are intra-cycle and do not each count as cycle
failures against the consecutive-failures breaker.
