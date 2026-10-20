# Option 1 — change no persona, record the verdict per persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

No file under `agents/` changes. The proposal records a verdict for each of the six core personas and ties it to the baseline rule of ADR-0133 D8 (E3). The operator gets a dated, falsifiable "no change" record.

## Causal chain to the goal metric

The goal is to keep phase quality while the platform changes. No edit means no unmeasured shift in ship rate, retries, contract blocks or tokens. The verdict lists the conditions that can flip it.

## Quantified expected effect

0 persona edits. 0 added D8 measurement load (E3). The page holds 4 new Models API fields and 2 thinking-type lines (E2). None reaches a request that the loop sends (A2, E4).

## Cost and time to effect

One small document cycle (E9). The effect is immediate.

## Top two risks

1. A later CLI release passes a `thinking` setting through to the phase. Detection: the wave-boundary smoke test of `evolve cli update` and the next watch diff (A1).
2. The "no change" habit hides a real shift. Detection: the flip conditions in the recommendation.
