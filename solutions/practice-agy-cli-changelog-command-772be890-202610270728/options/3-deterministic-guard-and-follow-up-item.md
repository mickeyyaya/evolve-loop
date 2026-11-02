# Option 3 — answer in bridge code, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a bridge driver change that reads the `finished_reason` field of agy JSON output. Add no prompt text to any persona file (E6).

## Causal chain to the goal metric

The driver tells a cut-off answer from a complete one without a prompt rule. The loop does not read this field today (E2), so the gain depends on a failure that is not seen.

## Quantified expected effect

0 failures caught today (E1, E2). A gain occurs only if a bridge run ends with a partial answer.

## Cost and time to effect

One build cycle for the Go change and its tests, with no active failure mode (E7).

## Top two risks

1. The change is dead code without a live failure mode, and breaks the caller-proof rule.
2. The field is an upstream detail that can change, so the driver can break on a later release (A1).
