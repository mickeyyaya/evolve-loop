# Option 1 — change no persona, record the verdict per persona

All numbers cite [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes, for whom

No file under `agents/` changes.
The proposal records a verdict for each of the six core personas.
It ties the verdict to the baseline rule of ADR-0133 D8 (E5).
The operator gets a clear record that can be tested.

## Causal chain to the goal metric

The goal is to protect pipeline quality while the CLI updates.
No edit means no unmeasured drift in ship rate, contract blocks, or token costs.
The recorded verdict names the exact conditions that can flip it.

## Quantified expected effect

Zero persona edits.
Zero added D8 measurement overhead (E5).
The 34 lines across v2.26.0 (E2) and v2.25.0 (E3) describe desktop UI and host bug fixes.
None changes a rule that a persona states (A4).

## Cost and time to effect

One small document cycle (E10).
The effect is immediate.

## Top two risks

1. A runtime fix changes behavior in a way that smoke tests miss. Detection: wave-boundary tests of `evolve cli update` (A1).
2. The no-change verdict hides a genuine opportunity. Detection: the flip conditions in `recommendation.md`.
