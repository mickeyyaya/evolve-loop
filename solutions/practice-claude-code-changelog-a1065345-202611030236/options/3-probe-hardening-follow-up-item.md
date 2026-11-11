# Option 3 — file a follow-up item to harden the `claude -p` probes

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

The bridge health check gets a rule that maps a `claude -p` exit 1 on a full disk to a host fault. The operator sees a disk fault, not a CLI fault. No persona changes.

## Causal chain to the goal metric

A correct fault class avoids a wrong CLI downgrade or a retry loop. The probes are at `driver.go:31` and `wallcorroborate.go:19` (E4).

## Quantified expected effect

1 probe path touched. The effect is 0 until a disk fills. The disk guard of ADR-0133 already covers the cause (E2). The rate of this case has no source.

## Cost and time to effect

One build cycle (E7). The effect shows only on a full disk.

## Top two risks

1. The rule has no live target today, so it is dead code. Detection: no test reaches it from a real probe.
2. The rule can hide a real CLI fault as a disk fault. Detection: the probe output on the next failure.
