# Option 1 — Keep S1 dropped and record why in the plan

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Only the plan document changes: the S1 row, §8 step 2 and rollout stage 1 (E1). The operator reads one
place that says why the early check cannot be safe. No Go file changes.

## Causal chain to the goal metric

The goal is a pre-scan check that saves a scan without a side effect. At ship, HEAD equals base (E2). A
`RebaseNeeded` result sends the lane to recovery, which drops it (E3). A check that stages first breaks
the tamper guard (E4). Dropping S1 removes a check that cannot meet both limits, and S5 and S6 save the
scan (E1).

## Quantified expected effect

Saving: none from S1; the cache carries it (A2). Risk removed: 1 bypass of the tamper guard.

## Cost and time-to-effect

One document edit in this cycle. Effect is immediate.

## Top risks and early detection

1. The cache (S5, S6) slips and no scan saving exists. *Detection:* the plan's rollout exit rules.
2. A later reader reopens S1. *Detection:* the S1 row names the three blockers (E2–E4).
