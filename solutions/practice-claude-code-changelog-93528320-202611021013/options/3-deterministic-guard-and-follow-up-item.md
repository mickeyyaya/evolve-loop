# Option 3 — add a deterministic guard for Opus 4.8 fast mode

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Add a check that fails when a route names Opus 4.8 fast mode after 2026-11-30. The operator gets an early error instead of a runtime error.

## Causal chain to the goal metric

A removed mode returns an error and does not fall back (E1). A guard would catch a route that selects it.

## Quantified expected effect

0 live targets today. No file outside tests names Opus 4.8 or fast mode (E4).

## Cost and time to effect

One build cycle for code with no live use. The effect starts on 2026-11-30 at the earliest.

## Top two risks

1. Dead code that no route reaches. Detection: the caller proof at the build floor.
2. A guard for one dated deprecation grows stale. Detection: removal after 2026-11-30.
