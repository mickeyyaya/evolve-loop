# Option 1 — change no persona, record the verdict per persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

No file under `agents/` changes (E8). The proposal records a verdict for each of the six core personas. The verdict ties directly to the baseline rule of ADR-0133 D8 (E3). The operator gets a clear record that can be tested.

## Causal chain to the goal metric

The goal is to maintain phase quality while external prompting practices evolve. No edit means zero unmeasured variance in ship rate, retries, contract blocks, or tokens. The proposal lists the conditions that can flip the choice.

## Quantified expected effect

0 persona edits. 0 added D8 measurement load (E3). The guide lines touch prompt engineering practices as untrusted data (E1, E2). None changes a rule that a core persona prompt states (A4, E5).

## Cost and time to effect

One small document cycle (E9). The effect is immediate.

## Top two risks

1. A guide line signals a model shift that affects tool use. Detection: wave-boundary monitor tests and harness error telemetry (A1).
2. The no-change choice delays prompt improvements. Detection: the flip conditions in the recommendation file.
