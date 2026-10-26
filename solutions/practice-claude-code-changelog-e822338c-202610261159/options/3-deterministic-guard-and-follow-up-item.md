# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a check in `evolve cli update`. After an update, the smoke test scans the new changelog section for setting names the loop uses. Today the loop uses none of the names in E2 (E5). The check finds nothing to guard.

## Causal chain to the goal metric

A code check catches a break that a persona cannot catch, with no added prompt bytes. The chain is weak here: with no setting in use (E5), no break can occur.

## Quantified expected effect

0 breaks caught now (E5). A future gain occurs only if the loop starts to use one of these settings.

## Cost and time to effect

One build cycle for the check, plus a test, and it covers no live risk (E5, E9).

## Top two risks

1. The check is dead code until a setting is in use, and the builder caller-proof rule bans dead code.
2. The match on names gives false alarms on unrelated lines (A4).
