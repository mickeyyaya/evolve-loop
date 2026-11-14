# Option 3 — answer in bridge code, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one follow-up item for a bridge driver change that reads the ACP context usage report (E1). No prompt text changes in any persona (E4).

## Causal chain to the goal metric

The driver can warn before a context limit without a prompt rule. The loop has no failure of this kind in the record of this cycle (E2), so the gain is not shown.

## Quantified expected effect

0 failures caught today (E2). A gain needs a run that ends at a context limit.

## Cost and time to effect

One build cycle for the Go change and its tests (E6), with no live failure mode.

## Top two risks

1. The change is dead code without a live failure mode, and breaks the caller-proof rule.
2. The report is an upstream detail that can change, so the driver can break later (A1).
