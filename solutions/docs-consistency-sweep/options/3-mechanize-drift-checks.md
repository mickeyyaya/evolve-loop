# Option 3 — Mechanize: CI docs-lint plus a compiled zero-ship breaker

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

- A CI job (or a `go/acs/regression` predicate) fails on any of three conditions: a duplicate ADR
  number prefix, a relative markdown link to a missing ADR file, or a coverage-index summary that
  differs from its map tallies (E1, E3, E7). These are the DCS-01/03/16 shapes made permanent.
- A compiled `zero-ship` rule in the blocker breaker counts EMPTY as well as FAIL cycles, so the
  operator's 2-cycle halt becomes a machine halt next to the 3-cycle `consecutive-failures` rule
  (E6).
- The docs are still corrected as in Option 1, because the lint would be red on day one otherwise.

## Causal chain to the goal

Drift of these three kinds (E1, E3, E7) could no longer merge, and the halt would stop depending on
the operator watching (E6).

## Expected effect

It ends recurrence of the mechanically checkable drift classes (3 of the 9 findings: E1, E3, E7, E11). The other 6
are claims about code behaviour that no lint can judge (E9). The compiled halt would trip one cycle
before the current compiled ceiling on FAIL streaks, and it would catch pure-EMPTY streaks that only
the goal-stall escalation sees today (E6).

## Cost and time to effect

Several cycles. The breaker change touches `go/internal/core/` and the guards, which are
control-plane surfaces reserved for console/operator work, and it needs its own ADR and soak (A4).

## Risks and early detection

1. **A compiled 2-cycle halt over-fires on legitimate no-work cycles**, such as a drained queue.
   Detection: a shadow stage that logs "would halt" before it enforces.
2. **Scope.** The breaker is out of bounds for a document cycle (A4). Doing it here would breach
   ADR-0099's deliverable-kind contract. Detection: the build floor and the protected-surface guard
   would refuse it.
