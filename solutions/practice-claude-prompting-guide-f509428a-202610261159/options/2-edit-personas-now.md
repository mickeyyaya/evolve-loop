# Option 2 — edit the affected personas now

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Rewrite `agents/evolve-builder.md` and `agents/evolve-scout.md` now to remove emphatic capital words. Adopt calm phrasing and XML instructions immediately.

## Causal chain to the goal metric

New prompt phrasing aims to reduce tool overtriggering and token waste. However, the causal chain needs a baseline to prove a benefit.

## Quantified expected effect

Unknown. No baseline exists (E4: M2 has not shipped). Therefore, no gain or loss can be measured (A3).

## Cost and time to effect

One persona edit plus 10 + 10 cycles of D8 measurements for each persona (E3). With two personas, ADR-0133 D7 is also broken.

## Top two risks

1. The edit breaks ADR-0133 decisions D7 and D8 (E3), so a regression cannot be traced.
2. Unmeasured prompt edits can degrade instruction following without telemetry (A3).
