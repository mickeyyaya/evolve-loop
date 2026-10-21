# Option 1 — change no persona, record the verdict per persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

No file under `agents/` changes. The proposal records a verdict for each of the six core personas and ties it to the baseline rule of ADR-0133 D8 (E3). The operator gets a dated record that can be proved wrong.

## Causal chain to the goal metric

The goal is to keep phase quality while the CLI changes. No edit means no unmeasured shift in ship rate, retries, contract blocks or tokens. The verdict lists the lines that can flip it.

## Quantified expected effect

0 persona edits. 0 added D8 measurement load (E3). The 39 bullets of E1 hold 6 lines that touch the loop as CLI behavior (E2). None changes a rule a persona states (A4, E5).

## Cost and time to effect

One small document cycle (E9). The effect is immediate.

## Top two risks

1. A line in E2 changes behavior without notice. Detection: the wave-boundary smoke test of `evolve cli update` and the next watch diff (A1).
2. The "no change" habit hides a real shift. Detection: the flip conditions in the recommendation.
