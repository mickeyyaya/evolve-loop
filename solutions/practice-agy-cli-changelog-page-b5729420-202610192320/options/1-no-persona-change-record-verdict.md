# Option 1 — change no persona, record the verdict per persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

No file under `agents/` changes. The proposal records a verdict for each of the six core personas and ties it to the baseline rule of ADR-0133 D8 (E8). The operator gets a dated record that can be proved wrong.

## Causal chain to the goal metric

The goal is to keep phase quality while the CLI changes. No edit means no unmeasured shift in ship rate, retries, contract blocks or tokens. The verdict names the lines that can flip it.

## Quantified expected effect

0 persona edits. 0 added D8 measurement load (E8). The 63 lines of E4 hold 10 lines that touch the loop's run behavior (E5). None changes a rule that a persona states (A4).

## Cost and time to effect

One small document cycle (E9). The effect is immediate.

## Top two risks

1. A line of E5 changes behavior in a way the smoke test misses. Detection: the wave-boundary smoke test of `evolve cli update` and the next watch diff (A1).
2. The "no change" habit hides a real shift. Detection: the flip conditions in the recommendation.
