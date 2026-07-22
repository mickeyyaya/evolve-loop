# ADR-0076: Convergence architecture — from inspect-and-restart to build-in-and-continue

- **Status:** Accepted (operator, 2026-07-23); slice B implemented with this ADR,
  slices A/C/D queued top-band as the `convergence-2026-07` campaign
- **Context evidence:** batches 6–8 (cycles 1044–1068)

## Context

After the July hardening campaign eliminated pipeline false-REDs entirely
(batch-8: every FAIL honest and fully evidenced), the pass rate stayed ~45%
against a projected ~90%. Every incident-level fix landed; the aggregate barely
moved. That signature — fixes land, rate doesn't — indicates an architectural
cause, and the failed-cycle data isolates it:

- The dominant honest-FAIL shape is **correct core work with a starved
  verification tail** (5 batch-8 builds rejected for unnamed exports — a
  mechanical, fully-specified step the builder ran out of room to do; the
  entire build for cycle-1056, including both corrections, spanned ~10 wall
  minutes on a structural-band item).
- The same items land at ~100% when the operator console works them —
  **not a capability gap but an economics gap**: the console iterates with
  retained state and unbounded budget; a lane gets one fixed-budget cold
  attempt.
- The queue is **survivorship-concentrated**: easy items drained weeks ago;
  what remains is precisely the work that does not fit one cycle's budget.
- Repeat attempts (role-lessons ×3, apicover-parity ×3, ship-stage ×2)
  restart cold and re-fail the same way: retries re-burn discovery, then
  starve the same tail.

Four compounding design properties produce this:
1. **Uniform budgets × non-uniform tasks** — identical phase windows and
   correction counts regardless of task difficulty.
2. **Quality inspected-in post-hoc** — floors reject after handoff; nothing
   requires the builder to present the checks green as part of "done";
   correction rounds share the dying session's budget.
3. **Restart-on-fail discards working state** — within-cycle correction
   exists; between-cycle continuation does not.
4. **Tier inversion under difficulty** — deep-tier audit reliably catches what
   balanced-tier build reliably cannot finish on hard items, with no
   escalation on retry.

The July fixes made failure cheaper, faster, and better-evidenced; none
changed the probability that a hard item converges in one attempt. This ADR
targets that probability.

## Decision

**A. Difficulty-conditioned budgets** *(queued: `difficulty-budgets-v2`)* —
triage's difficulty estimate drives multipliers for the build window,
correction rounds, and per-phase artifact timeouts. The plumbing exists as of
cycle-1057 (`phase_artifact_timeout_s` per-phase map); this slice adds the
difficulty signal and the multiplier policy (compiled defaults,
policy-overridable, no flags).

**B. Green-before-handoff** *(implemented with this ADR)* — `evolve selfcheck
build` gives the builder the floor's EXACT checks (`core.DefaultBuildFloorChecks`)
as an in-session pre-flight; the builder agent contract mandates running it
until green before declaring done. The floor remains unchanged as the
trust-boundary backstop (trust but verify) — the change is where the fixing
happens: inside the builder's own loop and budget, not in post-hoc salvage
windows.

**C. Continuation-on-fail** *(queued: `continuation-on-fail`)* — a FAILed
cycle with a salvageable preserved worktree requeues as a **continuation**
task binding that worktree: the next attempt resumes from the prior state plus
the failure's findings instead of restarting cold. Extends the fleet-rebase
carry-forward screen from *landing* work to *starting* work; S5 quarantine
ceilings still bound total attempts. This is the deepest change — it gives
lanes the same convergence property the console demonstrates.

**D. Recurrence-driven tier escalation** *(queued: `retry-tier-escalation`)* —
an item with `failure_count ≥ 1` routes its next build to the deep tier; the
failure counters (S5 chain) and per-phase routing seams both exist.

## Consequences

- Hard items consume more budget per attempt (A) and fewer attempts (C, D) —
  net token cost per *landed* item drops (a 12M-token FAIL-and-restart costs
  more than one 1.5× budget attempt that converges).
- The floor's rejection rate becomes a true anomaly signal again (B moves the
  routine catch in-session), restoring meaning to build-floor telemetry.
- Judgment boundaries are untouched: audit/adversarial phases stay
  non-remediable, tiers stay adversarially split; C never resumes past an
  audit verdict — it resumes *work*, not *grades*.

## Rejected alternatives

- **Raise correction rounds** — more post-hoc salvage windows inside the same
  dying budget; treats the symptom B fixes properly.
- **Shrink task scope globally** — fights triage's batching economics and
  doesn't help the genuinely-large structural band.
- **Route all hard work to the console** — abandons the product's purpose;
  the console's advantage (state + budget) is reproducible mechanically (A+C).
