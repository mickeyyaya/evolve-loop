# Option 2 — edit the personas now

All numbers cite [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes, for whom

Add rules to the builder and scout personas for the v2.26.0 and v2.25.0 updates (E2, E3).
Instruct personas about socket retry behavior and docked prompt handling.

## Causal chain to the goal metric

New prompt rules direct phase agents to rely on new CLI features.
This path expects fewer retries.
The chain requires a baseline to measure any improvement.

## Quantified expected effect

Unknown effect.
No baseline exists (E6), so no gain or loss can be measured (A3).

## Cost and time to effect

One persona edit plus 10 plus 10 cycles of D8 measures for each persona (E5).
Editing multiple personas also breaks ADR-0133 D7: one persona per change.

## Top two risks

1. The change violates ADR-0133 D7 and D8 (E5), so quality changes have no measurable cause.
2. Socket auto-retry is an Antigravity App desktop fix that does not reach the headless agy CLI (E2). The new prompt rules add redundant tokens without any underlying CLI change.
