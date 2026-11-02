# Option 2 — edit the builder persona now

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Shorten the tool-failure recovery text in `agents/evolve-builder.md` now, because agy 1.4.8 passes failed tool calls to the agent as errors (E2).

## Causal chain to the goal metric

A shorter prompt can save tokens. The causal chain needs a baseline to prove a benefit, and none exists.

## Quantified expected effect

Unknown. Milestone M2 has not shipped (E4), so no gain or loss can be measured (A3).

## Cost and time to effect

One persona edit plus 10 + 10 cycles of D8 measures (E3).

## Top two risks

1. The edit breaks the D8 baseline rule (E3), so a regression cannot be traced.
2. The recovery text covers other CLI families than agy, and a trim can degrade them (A4).
