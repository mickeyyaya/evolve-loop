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

- `PhaseRetro` already permitted `PhaseTDD` (alongside `PhaseShip` and `PhaseEnd`; the row
  also carries `PhaseAudit`, added earlier by ADR-0086's bookkeeping regrade), and
  `recoveryTarget` already resolved control-phase recovery targets from config.
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

**Known limitation, MEASURED rather than estimated.** Repair eligibility depends on
`disposition.json`, a retro deliverable. Two consequences, and the second is the larger one:

1. The first rejection still pays a full retrospective before any repair can be decided.
2. **The feature is unreachable on most failures.** Sampling the 16 most recent failed
   cycles in the runtime plane: 10 carry NO `disposition.json` at all, 3 record
   `false-rejection` (correctly not repairable), and only 3 record `legit-rejection`.
   Adding wave 3's three, roughly 6 of 19 — about one failure in three at best, and that
   is the ceiling, not the observed rate.

This is deliberate: the rule is conservative by construction and absence of evidence must
never grant a repair. But it means the honest claim for this ADR is "repair is available
when the retro says the rejection was task-level", NOT "audit failures are now repaired".
Cycle-1576 (2026-08-27) is a live instance — a legitimate H1 HIGH catch, no disposition
written, so no repair.

The lever is NOT widening eligibility, which would trade away the safety property this ADR
exists to protect. It is `disposition.json` being written when it is supposed to be:
`VerifyDisposition` is documented as fail-HARD ("retro cannot complete" without it), yet it
is routinely absent on failed cycles. That gap is filed separately.

**What this does not change.** Ship-time audit binding is untouched: each repair attempt
writes its own auditor ledger entry and ship binds the newest run-scoped one
(ADR-0084/#503/#504). Repair attempts are intra-cycle and do not each count as cycle
failures against the consecutive-failures breaker.

## Adversarial review found two more (and they are the same shape)

A go-reviewer pass on the implementation returned **BLOCK**. Both findings were
composition-root/trust-boundary gaps invisible to the pure-function tests:

- **The repair decision trusted `disposition.json` without the identity check the
  codebase already built for it.** `crossCheckAgainstDigest` exists specifically to stop
  an agent inventing a failure identity, and `readDispositionLegitimacy` read straight
  past it. A fabricated, stale, or copied disposition could therefore convert a genuine
  ADR-0072 system-failure HALT into a granted repair — and the ordinary repair grant was
  decided by the same untrusted read. Fixed: legitimacy is trusted only when the
  fingerprint/recurrence agree with `failure-digest.json` AND the disposition names THIS
  cycle. Unverifiable is not verified-good; both refusals return `""`.
- **The operator cap never reached production.** A dedicated field and Option existed,
  worked in tests, and were called from no composition root — so
  `workflow.max_audit_repair_attempts`, including the documented `0` off-switch, was
  inert. Fixed by DELETING the parallel surface: the cap now rides the single
  `workflowConfig` that `cmd_cycle.go` already injects, like every sibling knob.

Two MEDIUMs were also real: the repair brief leaked into later unrelated `tdd`/`build`
dispatches, because `AuditRepairAttempts` is a monotonic counter rather than a
"currently repairing" flag (now `AuditRepairActive`, cleared at the repair's own audit);
and the resume path never seeded the brief at all, so a cycle that crashed mid-repair
burned an attempt and rebuilt blind — while two comments claimed the paths "cannot
diverge". A re-review then caught the FIX for that one calling the seeder with the previous
phase rather than the dispatched one, leaving it wired and inert; both the call and its
argument are now guarded.

## Verification lesson (three defects, one shape)

Building this produced six defects, and they rhyme. Each was **green everywhere the
author looked and red on a path the author had not looked at**:

1. **The router ate the repair grant.** Every `decideAfterRetro` test passed while the
   ROUTED surface, `decideAfterRetroRouted`, handed the granted branch to the strategy,
   which overrode it to `end`. There are in fact three call surfaces, not one — routed
   live-loop, non-routed live-loop, and resume — and only the first passes through the
   router at all; `EVOLVE_DYNAMIC_ROUTING=advisory` is the default, so it is the one
   production usually takes. The sibling bookkeeping regrade already returns
   above the router for exactly this reason — the precedent existed and was not read
   closely enough. Caught only by writing a test against the LIVE path.
2. **A new `CycleState` field.** Caught by `bytestability_test.go`'s additive-keys guard.
3. **Two new exported symbols.** Caught by the Phase-5 apicover gate — and NOT by the
   test tier, which was fully green. The author had run `-race` and `-tags integration`
   *separately and per-package*; CI runs them combined module-wide and then gates on the
   resulting coverage profile. The gate was never exercised locally at all.

A sixth arrived from the FIX for the fourth, and it sharpens the rule. The resume-path
seeding was added — and passed its guard — while calling `seedAuditRepairContext(ctxSnap,
current, cs)`. At that point in the loop `current` is the phase that ran in the PREVIOUS
iteration; every sibling line in the same block keys on `next`. So the first TDD dispatch
after a resumed repair grant (Retro→TDD, precisely the crash-resume case the fix existed
for) saw `repairSeededPhase(PhaseRetro) == false` and rebuilt blind. The guard passed
because it was a `strings.Contains(body, "seedAuditRepairContext(")` scan: **it proved the
call existed, not that its arguments were right.** The guard now asserts the dispatched
phase is the argument, and that assertion kills the exact mutation that slipped through.

The generalizable rule: **a green test proves the path the test takes, and nothing about
the path production takes** — and a source-scan guard proves presence, not correctness. For a decision that has both a deterministic and a routed
entry point, pin the routed one; for a change that adds state or exported surface, run the
repo's own guards rather than inferring from the test suite. Two of these three were caught
by guards this repo had already built — which is the argument for keeping them.

Defect 1 is the asymmetric one, and worth naming: it was caught by a bespoke regression
test written for this one decision point, not by any repo-wide mechanism. Nothing would
catch the same routed-versus-direct duality at the NEXT decision point. Unlike the
additive-keys guard and the apicover gate, that rule is enforced by discipline alone.
