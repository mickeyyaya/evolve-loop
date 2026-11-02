# Option 1 — change no persona, record the verdict per persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

No file under `agents/` changes (E6). The research note records one verdict for each of the six core personas, tied to ADR-0133 D8 (E3). The operator gets a record that a grader can test (E2).

## Causal chain to the goal metric

The goal is stable phase quality while the agy CLI changes. No edit adds no unmeasured variance in ship rate, retries or tokens. The recommendation lists the conditions that flip the choice.

## Quantified expected effect

0 persona edits and 0 added D8 measure load (E3). The ten releases (E1) change no rule that a core persona states (A4, E5).

## Cost and time to effect

One trivial document cycle (E7). The effect is immediate.

## Top two risks

1. A release changes tool behavior that a persona relies on. Detection: harness error telemetry and the next wave-boundary run of the command (A1).
2. The no-change choice delays a builder trim. Detection: the flip conditions in the recommendation.
