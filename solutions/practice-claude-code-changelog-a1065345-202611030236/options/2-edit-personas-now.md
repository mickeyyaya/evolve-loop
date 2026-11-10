# Option 2 — edit personas now to match the release

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Two or more files under `agents/` change, for example a rule about re-reads (the removed Read stub) and one about effort. The phases that load those personas see new text.

## Causal chain to the goal metric

New text will state the new CLI behavior. The release adds no behavior a persona must rely on, so the chain has no cause to fix (E1, A4).

## Quantified expected effect

No gain can be proved. The baseline is missing (E3, A3). Decision 5 of ADR-0133 requires a before and after table (E2).

## Cost and time to effect

10 + 10 cycles of measures per persona (E7). The first proof arrives after about 20 cycles.

## Top two risks

1. An unmeasured edit lowers ship rate. Detection: none, because no baseline exists (E3).
2. The edit breaks the one-persona rule of ADR-0133 (E2). Detection: the docs review of the diff.
